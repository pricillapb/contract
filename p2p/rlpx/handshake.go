// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package rlpx

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"fmt"
	"hash"
	"io"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ethereum/go-ethereum/crypto/sha3"
)

const (
	maxUint24 = ^uint32(0) >> 8

	sskLen = 16 // ecies.MaxSharedKeyLength(pubKey) / 2
	sigLen = 65 // elliptic S256
	pubLen = 64 // 512 bit pubkey in uncompressed representation without format byte
	shaLen = 32 // hash length (for nonce etc)

	authMsgLen  = sigLen + shaLen + pubLen + shaLen + 1
	authRespLen = pubLen + shaLen + 1

	eciesBytes     = 65 + 16 + 32
	encAuthMsgLen  = authMsgLen + eciesBytes  // size of the final ECIES payload sent as initiator's handshake
	encAuthRespLen = authRespLen + eciesBytes // size of the final ECIES payload sent as receiver's handshake
)

// encHandshake contains the state of the encryption handshake.
type encHandshake struct {
	initiator            bool
	remotePub            *ecies.PublicKey  // remote-pubk
	initNonce, respNonce []byte            // nonce
	randomPrivKey        *ecies.PrivateKey // ecdhe-random
	remoteRandomPub      *ecies.PublicKey  // ecdhe-random-pubk
}

// secrets represents the connection secrets
// which are negotiated during the encryption handshake.
type secrets struct {
	RemoteID              *ecdsa.PublicKey
	AES, MAC              []byte
	EgressMAC, IngressMAC hash.Hash
	Token                 []byte
}

// secrets is called after the handshake is completed.
// It extracts the connection secrets from the handshake values.
func (h *encHandshake) secrets(auth, authResp []byte) (secrets, error) {
	ecdheSecret, err := h.randomPrivKey.GenerateShared(h.remoteRandomPub, sskLen, sskLen)
	if err != nil {
		return secrets{}, err
	}

	// derive base secrets from ephemeral key agreement
	sharedSecret := crypto.Sha3(ecdheSecret, crypto.Sha3(h.respNonce, h.initNonce))
	aesSecret := crypto.Sha3(ecdheSecret, sharedSecret)
	s := secrets{
		RemoteID: h.remotePub.ExportECDSA(),
		AES:      aesSecret,
		MAC:      crypto.Sha3(ecdheSecret, aesSecret),
		Token:    crypto.Sha3(sharedSecret),
	}

	// setup sha3 instances for the MACs
	mac1 := sha3.NewKeccak256()
	mac1.Write(xor(s.MAC, h.respNonce))
	mac1.Write(auth)
	mac2 := sha3.NewKeccak256()
	mac2.Write(xor(s.MAC, h.initNonce))
	mac2.Write(authResp)
	if h.initiator {
		s.EgressMAC, s.IngressMAC = mac1, mac2
	} else {
		s.EgressMAC, s.IngressMAC = mac2, mac1
	}

	return s, nil
}

func (h *encHandshake) ecdhShared(prv *ecdsa.PrivateKey) ([]byte, error) {
	return ecies.ImportECDSA(prv).GenerateShared(h.remotePub, sskLen, sskLen)
}

// initiatorEncHandshake negotiates a session token on conn.
// it should be called on the dialing side of the connection.
//
// prv is the local client's private key.
// token is the token from a previous session with this node.
func initiatorEncHandshake(conn io.ReadWriter, prv *ecdsa.PrivateKey, remoteID *ecdsa.PublicKey, token []byte) (s secrets, err error) {
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
	return h.secrets(auth, response)
}

func newInitiatorHandshake(remoteID *ecdsa.PublicKey) (*encHandshake, error) {
	// generate random initiator nonce
	n := make([]byte, shaLen)
	if _, err := rand.Read(n); err != nil {
		return nil, err
	}
	// generate random keypair to use for signing
	randpriv, err := ecies.GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		return nil, err
	}
	h := &encHandshake{
		initiator:     true,
		remotePub:     ecies.ImportECDSAPublic(remoteID),
		initNonce:     n,
		randomPrivKey: randpriv,
	}
	return h, nil
}

// authMsg creates an encrypted initiator handshake message.
func (h *encHandshake) authMsg(prv *ecdsa.PrivateKey, token []byte) ([]byte, error) {
	var tokenFlag byte
	if token == nil {
		// no session token found means we need to generate shared secret.
		// ecies shared secret is used as initial session token for new peers
		// generate shared key from prv and remote pubkey
		var err error
		if token, err = h.ecdhShared(prv); err != nil {
			return nil, err
		}
	} else {
		// for known peers, we use stored token from the previous session
		tokenFlag = 0x01
	}

	// sign known message:
	//   ecdh-shared-secret^nonce for new peers
	//   token^nonce for old peers
	signed := xor(token, h.initNonce)
	signature, err := crypto.Sign(signed, h.randomPrivKey.ExportECDSA())
	if err != nil {
		return nil, err
	}

	// encode auth message
	// signature || sha3(ecdhe-random-pubk) || pubk || nonce || token-flag
	msg := make([]byte, authMsgLen)
	n := copy(msg, signature)
	n += copy(msg[n:], crypto.Sha3(exportPubkey(&h.randomPrivKey.PublicKey)))
	n += copy(msg[n:], crypto.FromECDSAPub(&prv.PublicKey)[1:])
	n += copy(msg[n:], h.initNonce)
	msg[n] = tokenFlag

	// encrypt auth message using remote-pubk
	return ecies.Encrypt(rand.Reader, h.remotePub, msg, nil, nil)
}

// decodeAuthResp decode an encrypted authentication response message.
func (h *encHandshake) decodeAuthResp(auth []byte, prv *ecdsa.PrivateKey) error {
	msg, err := crypto.Decrypt(prv, auth)
	if err != nil {
		return fmt.Errorf("could not decrypt auth response (%v)", err)
	}
	h.respNonce = msg[pubLen : pubLen+shaLen]
	h.remoteRandomPub, err = importPublicKey(msg[:pubLen])
	if err != nil {
		return err
	}
	// ignore token flag for now
	return nil
}

// receiverEncHandshake negotiates a session token on conn.
// it should be called on the listening side of the connection.
//
// prv is the local client's private key.
// token is the token from a previous session with this node.
func receiverEncHandshake(conn io.ReadWriter, prv *ecdsa.PrivateKey, token []byte) (s secrets, err error) {
	// read remote auth sent by initiator.
	auth := make([]byte, encAuthMsgLen)
	if _, err := io.ReadFull(conn, auth); err != nil {
		return s, err
	}
	h, err := decodeAuthMsg(prv, token, auth)
	if err != nil {
		return s, err
	}

	// send auth response
	resp, err := h.authResp(prv, token)
	if err != nil {
		return s, err
	}
	if _, err = conn.Write(resp); err != nil {
		return s, err
	}

	return h.secrets(auth, resp)
}

func decodeAuthMsg(prv *ecdsa.PrivateKey, token []byte, auth []byte) (*encHandshake, error) {
	var err error
	h := new(encHandshake)
	// generate random keypair for session
	h.randomPrivKey, err = ecies.GenerateKey(rand.Reader, crypto.S256(), nil)
	if err != nil {
		return nil, err
	}
	// generate random nonce
	h.respNonce = make([]byte, shaLen)
	if _, err = rand.Read(h.respNonce); err != nil {
		return nil, err
	}

	msg, err := crypto.Decrypt(prv, auth)
	if err != nil {
		return nil, fmt.Errorf("could not decrypt auth message (%v)", err)
	}

	// decode message parameters
	// signature || sha3(ecdhe-random-pubk) || pubk || nonce || token-flag
	h.initNonce = msg[authMsgLen-shaLen-1 : authMsgLen-1]
	h.remotePub, err = importPublicKey(msg[sigLen+shaLen : sigLen+shaLen+pubLen])
	if err != nil {
		return nil, fmt.Errorf("bad remote identity: %v", err)
	}

	// recover remote random pubkey from signed message.
	if token == nil {
		// TODO: it is an error if the initiator has a token and we don't. check that.

		// no session token means we need to generate shared secret.
		// ecies shared secret is used as initial session token for new peers.
		// generate shared key from prv and remote pubkey.
		if token, err = h.ecdhShared(prv); err != nil {
			return nil, err
		}
	}
	signedMsg := xor(token, h.initNonce)
	remoteRandomPub, err := secp256k1.RecoverPubkey(signedMsg, msg[:sigLen])
	if err != nil {
		return nil, err
	}

	// validate the sha3 of recovered pubkey
	remoteRandomPubMAC := msg[sigLen : sigLen+shaLen]
	shaRemoteRandomPub := crypto.Sha3(remoteRandomPub[1:])
	if !bytes.Equal(remoteRandomPubMAC, shaRemoteRandomPub) {
		return nil, fmt.Errorf("sha3 of recovered ephemeral pubkey does not match checksum in auth message")
	}

	h.remoteRandomPub, _ = importPublicKey(remoteRandomPub)
	return h, nil
}

// authResp generates the encrypted authentication response message.
func (h *encHandshake) authResp(prv *ecdsa.PrivateKey, token []byte) ([]byte, error) {
	// responder auth message
	// E(remote-pubk, ecdhe-random-pubk || nonce || 0x0)
	resp := make([]byte, authRespLen)
	n := copy(resp, exportPubkey(&h.randomPrivKey.PublicKey))
	n += copy(resp[n:], h.respNonce)
	if token == nil {
		resp[n] = 0
	} else {
		resp[n] = 1
	}
	// encrypt using remote-pubk
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
