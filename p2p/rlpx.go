// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with go-ethereum.  If not, see <http://www.gnu.org/licenses/>.

package p2p

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	maxUint24 = ^uint32(0) >> 8

	sskLen   = 16 // ecies.MaxSharedKeyLength(pubKey) / 2
	sigLen   = 65 // elliptic S256
	pubLen   = 64 // 512 bit pubkey in uncompressed representation without format byte
	nonceLen = 32

	authMsgLen  = 2*pubLen + nonceLen
	authRespLen = pubLen + nonceLen

	kdfSharedDataName = "rlpx handshake"
	kdfSharedDataLen  = len(kdfSharedDataName) + 2*pubLen + 2*nonceLen

	// encryption handshake payload sizes
	eciesOverhead  = 65 + 16 + 32
	encAuthMsgLen  = authMsgLen + eciesOverhead
	encAuthRespLen = authRespLen + eciesOverhead

	// This is the maximum number of frames that are encrypted and
	// sent with the same symmetric encryption key.
	frameKeyLimit = 100

	// Total timeout for encryption handshake and protocol
	// handshake in both directions.
	handshakeTimeout = 5 * time.Second

	// This is the timeout for sending the disconnect reason.
	// This is shorter than the usual timeout because we don't want
	// to wait if the connection is known to be bad anyway.
	discWriteTimeout = 1 * time.Second
)

// rlpx is the transport protocol used by actual (non-test) connections.
// It wraps the frame encoder with locks and read/write deadlines.
type rlpx struct {
	fd       net.Conn
	rmu, wmu sync.Mutex
	rw       *rlpxFrameRW
}

func newRLPX(fd net.Conn) transport {
	return &rlpx{fd: fd}
}

func (t *rlpx) ReadMsg() (Msg, error) {
	t.rmu.Lock()
	defer t.rmu.Unlock()
	t.fd.SetReadDeadline(time.Now().Add(frameReadTimeout))
	return t.rw.ReadMsg()
}

// WriteMsg sends a single message on the connection. The remote end
// must receive the entire message within frameWriteTimeout.
func (t *rlpx) WriteMsg(msg Msg) error {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	t.fd.SetWriteDeadline(time.Now().Add(frameWriteTimeout))
	return t.rw.WriteMsg(msg)
}

// close shuts down the connection. If err is a DiscReason and the
// connection has already executed key-establishment, the remote end
// receives a devp2p disconnect message before the connection is
// closed.
func (t *rlpx) close(err error) {
	t.wmu.Lock()
	defer t.wmu.Unlock()
	if t.rw != nil {
		if r, ok := err.(DiscReason); ok && r != DiscNetworkError {
			t.fd.SetWriteDeadline(time.Now().Add(discWriteTimeout))
			SendItems(t.rw, discMsg, r)
		}
	}
	t.fd.Close()
}

// doEncHandshake runs the protocol handshake using authenticated
// messages. the protocol handshake is the first authenticated message
// and also verifies whether the encryption handshake 'worked' and the
// remote side actually provided the right public key.
func (t *rlpx) doProtoHandshake(our *protoHandshake) (their *protoHandshake, err error) {
	// Writing our handshake happens concurrently, we prefer
	// returning the handshake read error. If the remote side
	// disconnects us early with a valid reason, we should return it
	// as the error so it can be tracked elsewhere.
	werr := make(chan error, 1)
	go func() { werr <- Send(t.rw, handshakeMsg, our) }()
	if their, err = readProtocolHandshake(t.rw, our); err != nil {
		<-werr // make sure the write terminates too
		return nil, err
	}
	if err := <-werr; err != nil {
		return nil, fmt.Errorf("write error: %v", err)
	}
	return their, nil
}

func readProtocolHandshake(rw MsgReader, our *protoHandshake) (*protoHandshake, error) {
	msg, err := rw.ReadMsg()
	if err != nil {
		return nil, err
	}
	if msg.Size > baseProtocolMaxMsgSize {
		return nil, fmt.Errorf("message too big")
	}
	if msg.Code == discMsg {
		// Disconnect before protocol handshake is valid according to the
		// spec and we send it ourself if the posthanshake checks fail.
		// We can't return the reason directly, though, because it is echoed
		// back otherwise. Wrap it in a string instead.
		var reason [1]DiscReason
		rlp.Decode(msg.Payload, &reason)
		return nil, reason[0]
	}
	if msg.Code != handshakeMsg {
		return nil, fmt.Errorf("expected handshake, got %x", msg.Code)
	}
	var hs protoHandshake
	if err := msg.Decode(&hs); err != nil {
		return nil, err
	}
	// validate handshake info
	if hs.Version != our.Version {
		return nil, DiscIncompatibleVersion
	}
	if (hs.ID == discover.NodeID{}) {
		return nil, DiscInvalidIdentity
	}
	return &hs, nil
}

// doEncHandshake executes the initial key-establishment. If dial is
// non-nil, the local end of the connection is the initiator.
func (t *rlpx) doEncHandshake(prv *ecdsa.PrivateKey, dial *discover.Node) (discover.NodeID, error) {
	var (
		sec secrets
		err error
	)
	t.fd.SetDeadline(time.Now().Add(handshakeTimeout))
	if dial == nil {
		sec, err = recipientEncHandshake(t.fd, prv, nil)
	} else {
		sec, err = initiatorEncHandshake(t.fd, prv, dial.ID, nil)
	}
	if err != nil {
		return discover.NodeID{}, err
	}
	t.wmu.Lock()
	t.rw = newRLPXFrameRW(t.fd, sec)
	t.wmu.Unlock()
	return sec.RemoteID, nil
}

// encHandshake contains the state of the encryption handshake.
type encHandshake struct {
	initiator bool
	remoteID  discover.NodeID

	remotePub            *ecies.PublicKey  // remote-pubk
	initNonce, respNonce []byte            // nonce
	randomPrivKey        *ecies.PrivateKey // ecdhe-random
	remoteRandomPub      *ecies.PublicKey  // ecdhe-random-pubk
}

// secrets represents the connection secrets
// which are negotiated during the encryption handshake.
type secrets struct {
	Initiator  bool
	RemoteID   discover.NodeID
	AES1, AES2 []byte
	IV1, IV2   uint32
}

// secrets derives connection secrets from the handshake messages.
func (h *encHandshake) secrets(prv *ecdsa.PrivateKey) (secrets, error) {
	// Derive shared secret using ECDH.
	sharedSecret, err := h.randomPrivKey.GenerateShared(h.remoteRandomPub, sskLen, sskLen)
	if err != nil {
		return secrets{}, err
	}
	// Derive AES secrets from shared secret using the KDF.
	sharedData := h.kdfSharedData(prv)
	derivedSecret, err := ecies.ConcatKDF(sha3.NewKeccak256(), sharedSecret, sharedData, 64)
	if err != nil {
		return secrets{}, fmt.Errorf("KDF error: %v", err)
	}
	s := secrets{
		RemoteID: h.remoteID,
		AES1:     derivedSecret[:32],
		AES2:     derivedSecret[32:],
		IV1:      binary.BigEndian.Uint32(h.initNonce),
		IV2:      binary.BigEndian.Uint32(h.respNonce),
	}
	if h.initiator {
		s.AES1, s.AES2 = s.AES2, s.AES1
		s.IV1, s.IV2 = s.IV2, s.IV1
	}
	return s, nil
}

func (h *encHandshake) kdfSharedData(prv *ecdsa.PrivateKey) []byte {
	localID := crypto.FromECDSAPub(&prv.PublicKey)[1:]
	sharedData := make([]byte, kdfSharedDataLen)
	n := fillBytes(sharedData, []byte(kdfSharedDataName), h.initNonce, h.respNonce)
	if h.initiator {
		fillBytes(sharedData[n:], localID, h.remoteID[:])
	} else {
		fillBytes(sharedData[n:], h.remoteID[:], localID)
	}
	return sharedData
}

func (h *encHandshake) ecdhShared(prv *ecdsa.PrivateKey) ([]byte, error) {
	return ecies.ImportECDSA(prv).GenerateShared(h.remotePub, sskLen, sskLen)
}

func fillBytes(dst []byte, src ...[]byte) (n int) {
	for _, b := range src {
		nn := copy(dst[n:], b)
		n += nn
		if nn < len(b) {
			panic("dst too short")
		}
	}
	return n
}

// initiatorEncHandshake negotiates a session token on conn.
// It should be called on the dialing side of the connection.
//
// prv is the local client's private key.
// token is the token from a previous session with this node.
func initiatorEncHandshake(conn io.ReadWriter, prv *ecdsa.PrivateKey, remoteID discover.NodeID, token []byte) (s secrets, err error) {
	h, err := newInitiatorHandshake(remoteID)
	if err != nil {
		return s, err
	}
	auth, err := h.authMsg(prv, token)
	if err != nil {
		return s, err
	}
	if _, err = conn.Write(auth); err != nil {
		return s, err
	}

	response := make([]byte, encAuthRespLen)
	if _, err = io.ReadFull(conn, response); err != nil {
		return s, err
	}
	if err := h.decodeAuthResp(response, prv); err != nil {
		return s, err
	}
	return h.secrets(prv)
}

func newInitiatorHandshake(remoteID discover.NodeID) (*encHandshake, error) {
	// generate random initiator nonce
	n := make([]byte, nonceLen)
	if _, err := rand.Read(n); err != nil {
		return nil, err
	}
	// generate random keypair to use for signing
	randpriv, err := ecies.GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		return nil, err
	}
	rpub, err := remoteID.Pubkey()
	if err != nil {
		return nil, fmt.Errorf("bad remoteID: %v", err)
	}
	h := &encHandshake{
		initiator:     true,
		remoteID:      remoteID,
		remotePub:     ecies.ImportECDSAPublic(rpub),
		initNonce:     n,
		randomPrivKey: randpriv,
	}
	return h, nil
}

// authMsg creates an encrypted initiator handshake message.
func (h *encHandshake) authMsg(prv *ecdsa.PrivateKey, token []byte) ([]byte, error) {
	// encode auth message: S(initiator-privk, nonce,
	msg := make([]byte, authMsgLen)
	fillBytes(msg,
		exportPubkey(&h.randomPrivKey.PublicKey),
		crypto.FromECDSAPub(&prv.PublicKey)[1:],
		h.initNonce,
	)
	// encrypt auth message using remote-pubk
	return ecies.Encrypt(rand.Reader, h.remotePub, msg, nil, nil)
}

// decodeAuthResp decode an encrypted authentication response message.
func (h *encHandshake) decodeAuthResp(auth []byte, prv *ecdsa.PrivateKey) error {
	msg, err := crypto.Decrypt(prv, auth)
	if err != nil {
		return fmt.Errorf("could not decrypt auth response (%v)", err)
	}
	h.remoteRandomPub, err = importPublicKey(msg[:pubLen])
	if err != nil {
		return err
	}
	h.respNonce = msg[pubLen:]
	return nil
}

// recipientEncHandshake negotiates a session token on conn.
// it should be called on the listening side of the connection.
//
// prv is the local client's private key.
// token is the token from a previous session with this node.
func recipientEncHandshake(conn io.ReadWriter, prv *ecdsa.PrivateKey, token []byte) (s secrets, err error) {
	// Read auth message sent by initiator.
	auth := make([]byte, encAuthMsgLen)
	if _, err := io.ReadFull(conn, auth); err != nil {
		return s, err
	}
	h, err := decodeAuthMsg(prv, token, auth)
	if err != nil {
		return s, err
	}
	// Send auth response message.
	resp, err := h.authResp(prv, token)
	if err != nil {
		return s, err
	}
	if _, err = conn.Write(resp); err != nil {
		return s, err
	}

	return h.secrets(prv)
}

func decodeAuthMsg(prv *ecdsa.PrivateKey, token []byte, auth []byte) (*encHandshake, error) {
	var err error
	h := new(encHandshake)
	// decode initator message: ecdhe-random-pubk || pubk || nonce
	msg, err := crypto.Decrypt(prv, auth)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt auth message (%v)", err)
	}
	copy(h.remoteID[:], msg[pubLen:2*pubLen])
	rpub, err := h.remoteID.Pubkey()
	if err != nil {
		return nil, fmt.Errorf("bad remoteID: %#v", err)
	}
	h.remotePub = ecies.ImportECDSAPublic(rpub)
	h.remoteRandomPub, err = importPublicKey(msg[:pubLen])
	if err != nil {
		return nil, fmt.Errorf("invalid random public key: %v", err)
	}
	h.initNonce = msg[2*pubLen:]
	// generate random handshake keypair
	h.randomPrivKey, err = ecies.GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		return nil, err
	}
	// generate random nonce
	h.respNonce = make([]byte, nonceLen)
	if _, err = rand.Read(h.respNonce); err != nil {
		return nil, err
	}
	return h, nil
}

// authResp generates the encrypted authentication response message.
func (h *encHandshake) authResp(prv *ecdsa.PrivateKey, token []byte) ([]byte, error) {
	// E(remote-pubk, ecdhe-random-pubk || nonce)
	resp := append(exportPubkey(&h.randomPrivKey.PublicKey), h.respNonce...)
	return ecies.Encrypt(rand.Reader, h.remotePub, resp, nil, nil)
}

// importPublicKey unmarshals 512 bit public keys.
func importPublicKey(pubKey []byte) (*ecies.PublicKey, error) {
	var pubKey65 []byte
	switch len(pubKey) {
	case 64:
		// add 'uncompressed key' flag
		pubKey65 = append([]byte{0x04}, pubKey...)
	case 65:
		pubKey65 = pubKey
	default:
		return nil, fmt.Errorf("invalid public key length %v (expect 64/65)", len(pubKey))
	}
	// TODO: fewer pointless conversions
	return ecies.ImportECDSAPublic(crypto.ToECDSAPub(pubKey65)), nil
}

func exportPubkey(pub *ecies.PublicKey) []byte {
	if pub == nil {
		panic("nil pubkey")
	}
	return elliptic.Marshal(pub.Curve, pub.X, pub.Y)[1:]
}

func xor(one, other []byte) (xor []byte) {
	xor = make([]byte, len(one))
	for i := 0; i < len(one); i++ {
		xor[i] = one[i] ^ other[i]
	}
	return xor
}

var (
	// this is used in place of actual frame header data.
	// TODO: replace this when Msg contains the protocol type code.
	zeroHeader = []byte{0xC2, 0x80, 0x80}
	// sixteen zero bytes
	zero16 = make([]byte, 16)
)

// rlpxFrameRW implements a simplified version of RLPx framing.
// chunked messages are not supported and all headers are equal to
// zeroHeader.
//
// rlpxFrameRW is not safe for concurrent use from multiple goroutines.
type rlpxFrameRW struct {
	conn      io.ReadWriter
	initiator bool // if true, this end of the connection is the initiator.
	enc       cipher.AEAD
	dec       cipher.AEAD
	encIV     *ivgen
	decIV     *ivgen
}

func newRLPXFrameRW(conn io.ReadWriter, s secrets) *rlpxFrameRW {
	var (
		rw         = &rlpxFrameRW{conn: conn}
		encc, decc cipher.Block
		err        error
	)
	rw.encIV = newivgen(s.IV1)
	rw.decIV = newivgen(s.IV2)
	if encc, err = aes.NewCipher(s.AES1); err != nil {
		panic("invalid AES1: " + err.Error())
	}
	if decc, err = aes.NewCipher(s.AES2); err != nil {
		panic("invalid AES2: " + err.Error())
	}
	if rw.enc, err = cipher.NewGCM(encc); err != nil {
		panic("can't create GCM: " + err.Error())
	}
	if rw.dec, err = cipher.NewGCM(decc); err != nil {
		panic("can't create GCM: " + err.Error())
	}
	return rw
}

func (rw *rlpxFrameRW) resetSecrets(s secrets) {
	var (
		encc, decc cipher.Block
		err        error
	)
	rw.encIV = newivgen(s.IV1)
	rw.decIV = newivgen(s.IV2)
	if encc, err = aes.NewCipher(s.AES1); err != nil {
		panic("invalid AES1: " + err.Error())
	}
	if decc, err = aes.NewCipher(s.AES2); err != nil {
		panic("invalid AES2: " + err.Error())
	}
	if rw.enc, err = cipher.NewGCM(encc); err != nil {
		panic("can't create GCM: " + err.Error())
	}
	if rw.dec, err = cipher.NewGCM(decc); err != nil {
		panic("can't create GCM: " + err.Error())
	}
}

// writeRehandshake writes a devp2p message frame to the connection.
func (rw *rlpxFrameRW) WriteMsg(msg Msg) error {
	buf := new(bytes.Buffer)
	rlp.Encode(buf, msg.Code)
	io.Copy(buf, msg.Payload)
	if err := rw.writeHeader(0, 0, uint32(buf.Len())); err != nil {
		return err
	}
	// Messsage frames are encrypted with the AEAD cipher like the header.
	pad16(buf)
	buf.Grow(buf.Len() + rw.enc.Overhead())
	b := buf.Bytes()
	b = rw.enc.Seal(b[:0], rw.encIV.next(), b, nil)
	_, err := rw.conn.Write(b)
	return err
}

// writeHeader writes an encrypted frame header to the connection.
func (rw *rlpxFrameRW) writeHeader(protocolType, sequenceID uint16, size uint32) error {
	if size > maxUint24 {
		return errors.New("message size overflows uint24")
	}
	buf := new(bytes.Buffer)
	buf.Write(zero16[:3])
	putInt24(uint32(size), buf.Bytes())
	rlp.Encode(buf, []uint16{protocolType, sequenceID})
	pad16(buf)
	b := buf.Bytes()
	b = rw.enc.Seal(b[:0], rw.encIV.next(), b[:16], nil)
	_, err := rw.conn.Write(b)
	return err
}

func (rw *rlpxFrameRW) ReadMsg() (msg Msg, err error) {
	// read the header
	headbuf := make([]byte, 16+rw.dec.Overhead())
	if _, err = io.ReadFull(rw.conn, headbuf); err != nil {
		return msg, err
	}
	headbuf, err = rw.dec.Open(headbuf[:0], rw.decIV.next(), headbuf, nil)
	if err != nil {
		return msg, fmt.Errorf("header decryption failed (%v)", err)
	}
	fsize := readInt24(headbuf)
	// ignore protocol type for now

	// read the frame content
	var rsize = fsize // frame size rounded up to 16 byte boundary
	if padding := fsize % 16; padding > 0 {
		rsize += 16 - padding
	}
	framebuf := make([]byte, rsize+uint32(rw.dec.Overhead()))
	if _, err := io.ReadFull(rw.conn, framebuf); err != nil {
		return msg, err
	}
	framebuf, err = rw.dec.Open(framebuf[:0], rw.decIV.next(), framebuf, nil)
	if err != nil {
		return msg, fmt.Errorf("frame content decryption failed (%v)", err)
	}

	// decode message code
	content := bytes.NewReader(framebuf[:fsize])
	if err = rlp.Decode(content, &msg.Code); err != nil {
		return msg, err
	}
	msg.Size = uint32(content.Len())
	msg.Payload = content
	return msg, nil
}

// ivgen generates 96 bit deterministic IVs for AES-GCM.
type ivgen struct {
	ctr uint64
	buf []byte
}

func newivgen(rand uint32) *ivgen {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf, rand)
	return &ivgen{buf: buf}
}

func (g *ivgen) next() []byte {
	binary.BigEndian.PutUint64(g.buf[4:], g.ctr)
	g.ctr++
	return g.buf
}

func readInt24(b []byte) uint32 {
	return uint32(b[2]) | uint32(b[1])<<8 | uint32(b[0])<<16
}

func putInt24(v uint32, b []byte) {
	b[0] = byte(v >> 16)
	b[1] = byte(v >> 8)
	b[2] = byte(v)
}

func pad16(buf *bytes.Buffer) {
	if padding := buf.Len() % 16; padding > 0 {
		buf.Write(zero16[:16-padding])
	}
}
