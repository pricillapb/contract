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
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync"

	"github.com/ethereum/go-ethereum/rlp"
)

const (
	staticFrameSize     uint32 = 8 * 1024
	frameHeaderSize            = 16 // encoded header
	frameHeaderFullSize        = 32 // encoded header + MAC
)

var zero16 = make([]byte, 16)

var (
	ErrProtocolClaimTimeout = errors.New("protocol for pending message was not claimed in time")
	errUnexpectedChunkStart = errors.New("received chunk start header for existing transfer")
	errChunkTooLarge        = errors.New("chunk size larger than remaining message size")
)

// readLoop runs in its own goroutine for each connection,
// dispatching frames to protocols.
func readLoop(c *Conn) (err error) {
	defer func() {
		// When the loop ends, forward the error to all protocols so
		// their next ReadPacket fails. Active chunked transfers also
		// need to cancel immediately so shutdown is not delayed.
		c.mu.Lock()
		for _, p := range c.proto {
			p.readClose(err)
			for _, cr := range p.xfers {
				cr.close(err)
			}
		}
		c.readErr = err
		c.mu.Unlock()
	}()

	// Local cache of claimed protocols.
	proto := make(map[uint16]*Protocol)

	for {
		hdr, body, err := c.rw.readFrame()
		if err != nil {
			return err
		}
		// Grab the protocol, checking the local cache before
		// interacting with the claims machinery in Conn.
		p := proto[hdr.protocol]
		if p == nil {
			if p = c.waitForProtocol(hdr.protocol); p == nil {
				return ErrProtocolClaimTimeout
			}
			proto[p.id] = p
		}

		if cr := p.xfers[hdr.contextID]; cr != nil {
			// existing chunked transfer
			if hdr.chunkStart {
				return errUnexpectedChunkStart
			}
			end, err := cr.feed(body)
			if end {
				delete(p.xfers, hdr.contextID)
			}
			if err != nil {
				return err
			}
		} else {
			// new transfer
			len, r, err := frameToPacket(hdr, body)
			if err != nil {
				return err
			}
			if cr, ok := r.(*chunkedReader); ok {
				// Keep dispatching chunks for this protocol until the packet ends.
				p.xfers[hdr.contextID] = cr
			}
			p.newPacket <- packet{len, r}
		}
	}
}

// frameToPacket handles the initial frame for a new packet.
func frameToPacket(hdr frameHeader, body *bytes.Buffer) (len uint32, r io.Reader, err error) {
	len = uint32(body.Len())
	if !hdr.chunkStart || len == hdr.totalSize {
		// For regular frames, the frame is the packet.
		return len, body, nil
	}
	// For chunk start frames, the body reader is a chunkReader.
	if len > hdr.totalSize {
		return len, nil, fmt.Errorf("initial chunk size %d larger than total size %d")
	}
	cr := newChunkedReader(hdr.totalSize)
	cr.feed(body)
	return hdr.totalSize, cr, nil
}

// chunkedReader is the payload reader for chunked messages.
// chunks are appended to it as they are read from the connection.
// when the content of a chunk has been consumed, it returns to the
// buffer pool.
type chunkedReader struct {
	cond        *sync.Cond
	bufs        []*bytes.Buffer
	err         error
	readN, bufN uint32 // how much still needs to be read/buffered
}

func newChunkedReader(len uint32) *chunkedReader {
	return &chunkedReader{cond: sync.NewCond(new(sync.Mutex)), readN: len, bufN: len}
}

func (cr *chunkedReader) Read(rslice []byte) (int, error) {
	cr.cond.L.Lock()
	defer cr.cond.L.Unlock()

	if err := cr.waitFrame(); err != nil {
		return 0, err
	}
	nn := 0
	for _, buf := range cr.bufs {
		n, _ := buf.Read(rslice[nn:])
		nn += n
		if nn == len(rslice) {
			break
		}
	}
	cr.readN -= uint32(nn)
	cr.removeDrainedBuffers()
	return nn, nil
}

func (cr *chunkedReader) ReadByte() (byte, error) {
	cr.cond.L.Lock()
	defer cr.cond.L.Unlock()

	if err := cr.waitFrame(); err != nil {
		return 0, err
	}
	// TODO: make sure that frames are always non-empty.
	b, _ := cr.bufs[0].ReadByte()
	cr.removeDrainedBuffers()
	return b, nil
}

// blocks until at least one frame is available
func (cr *chunkedReader) waitFrame() error {
	for {
		if cr.err != nil {
			return cr.err
		}
		if cr.readN == 0 {
			return io.EOF
		}
		if len(cr.bufs) > 0 {
			break
		}
		cr.cond.Wait()
	}
	return nil
}

func (cr *chunkedReader) removeDrainedBuffers() {
	drained := 0
	for _, buf := range cr.bufs {
		if buf.Len() != 0 {
			break
		}
		drained++
	}
	cr.bufs = cr.bufs[:copy(cr.bufs, cr.bufs[drained:])]
}

func (cr *chunkedReader) close(err error) {
	cr.cond.L.Lock()
	cr.err = err
	cr.cond.Signal() // wake up Read
	cr.cond.L.Unlock()
}

func (cr *chunkedReader) feed(body *bytes.Buffer) (bool, error) {
	cr.cond.L.Lock()
	defer cr.cond.L.Unlock()
	ulen := uint32(body.Len())
	if ulen > cr.bufN {
		cr.err = errChunkTooLarge
		cr.cond.Signal() // wake up Read
		return true, cr.err
	}
	cr.bufN -= ulen
	cr.bufs = append(cr.bufs, body)
	cr.cond.Signal() // wake up Read
	return cr.bufN == 0, nil
}

// represents a frame header that has been read.
type frameHeader struct {
	protocol, contextID uint16
	chunkStart          bool   // initial frame of chunked message
	totalSize           uint32 // total number of bytes of chunked message
}

// header types for sending
type chunkStartHeader struct {
	Protocol, ContextID uint16
	TotalSize           uint32
}
type regularHeader struct {
	Protocol, ContextID uint16
}

func decodeHeader(b []byte) (h frameHeader, err error) {
	lc, rest, err := rlp.SplitList(b)
	if err != nil {
		return h, err
	}
	// This is silly. rlp.DecodeBytes errors for data
	// after the value, so we need to pass a slice
	// containing just the value.
	hlist := b[:len(b)-len(rest)]

	switch cnt, _ := rlp.CountValues(lc); cnt {
	case 1:
		var in struct{ Protocol uint16 }
		err = rlp.DecodeBytes(hlist, &in)
		h.protocol = in.Protocol
	case 2:
		var in regularHeader
		err = rlp.DecodeBytes(hlist, &in)
		h.protocol = in.Protocol
		h.contextID = in.ContextID
	case 3:
		var in chunkStartHeader
		err = rlp.DecodeBytes(hlist, &in)
		h.protocol = in.Protocol
		h.contextID = in.ContextID
		h.totalSize = in.TotalSize
		h.chunkStart = true
	default:
		err = fmt.Errorf("too many list elements")
	}
	return h, err
}

// frameRW implements the framed wire protocol.
type frameRW struct {
	conn      io.ReadWriter
	macCipher cipher.Block

	enc       cipher.Stream
	egressMAC hash.Hash

	headbuf    []byte
	dec        cipher.Stream
	ingressMAC hash.Hash
}

func newFrameRW(conn io.ReadWriter, s secrets) *frameRW {
	macc, err := aes.NewCipher(s.MAC)
	if err != nil {
		panic("invalid MAC secret: " + err.Error())
	}
	encc, err := aes.NewCipher(s.AES)
	if err != nil {
		panic("invalid AES secret: " + err.Error())
	}
	// we use an all-zeroes IV for AES because the key used
	// for encryption is ephemeral.
	iv := make([]byte, encc.BlockSize())
	return &frameRW{
		conn:       conn,
		enc:        cipher.NewCTR(encc, iv),
		dec:        cipher.NewCTR(encc, iv),
		macCipher:  macc,
		headbuf:    make([]byte, 32),
		egressMAC:  s.EgressMAC,
		ingressMAC: s.IngressMAC,
	}
}

// sends a frame on the connection. the body buffer must placeholder bytes
// for the encoded frame header and its MAC.
func (rw *frameRW) sendFrame(hdr interface{}, body *frameBuffer) error {
	wbuf := *body
	usize := uint32(len(wbuf))
	if usize < frameHeaderFullSize {
		panic(fmt.Sprintf("invalid body buffer, size < %d", frameHeaderFullSize))
	}
	if usize-frameHeaderFullSize > maxUint24 {
		return errors.New("frame size overflows uint24")
	}

	// Write and encrypt the frame header to the buffer.
	headbuf := wbuf[:frameHeaderSize]
	putInt24(headbuf, usize-frameHeaderFullSize)
	headbufAfterSize := headbuf[3:3]
	rlp.Encode(&headbufAfterSize, hdr)
	rw.enc.XORKeyStream(headbuf, headbuf)
	copy(wbuf[frameHeaderSize:], updateMAC(rw.egressMAC, rw.macCipher, headbuf))

	// Write and encrypt frame data to the buffer.
	wbuf.pad16()
	rw.enc.XORKeyStream(wbuf[frameHeaderFullSize:], wbuf[frameHeaderFullSize:])
	rw.egressMAC.Write(wbuf[frameHeaderFullSize:])
	fmacseed := rw.egressMAC.Sum(nil)
	wbuf = append(wbuf, zero16...)
	copy(wbuf[len(wbuf)-16:], updateMAC(rw.egressMAC, rw.macCipher, fmacseed))

	// Send the whole buffered frame on the socket.
	_, err := rw.conn.Write(wbuf)
	*body = wbuf
	return err
}

func (rw *frameRW) readFrame() (hdr frameHeader, body *bytes.Buffer, err error) {
	// Read the header and verify its MAC.
	if _, err := io.ReadFull(rw.conn, rw.headbuf); err != nil {
		return hdr, nil, err
	}
	shouldMAC := updateMAC(rw.ingressMAC, rw.macCipher, rw.headbuf[:16])
	if !hmac.Equal(shouldMAC, rw.headbuf[16:]) {
		return hdr, nil, errors.New("bad header MAC")
	}
	rw.dec.XORKeyStream(rw.headbuf[:16], rw.headbuf[:16])

	// Parse the header.
	fsize := readInt24(rw.headbuf)
	hdr, err = decodeHeader(rw.headbuf[3:])
	if err != nil {
		return hdr, nil, fmt.Errorf("invalid frame header: %v", err)
	}

	// Grab a buffer for the content.
	body = new(bytes.Buffer)
	var rsize = fsize // frame size rounded up to 16 byte boundary
	if padding := fsize % 16; padding > 0 {
		rsize += 16 - padding
	}
	if _, err := io.CopyN(body, rw.conn, int64(rsize)+16); err != nil {
		return hdr, nil, err
	}

	// Verify the body MAC and decrypt the content.
	fb := body.Bytes()
	mac, bb := fb[len(fb)-16:], fb[:len(fb)-16]
	rw.ingressMAC.Write(bb)
	fmacseed := rw.ingressMAC.Sum(nil)
	shouldMAC = updateMAC(rw.ingressMAC, rw.macCipher, fmacseed)
	if !hmac.Equal(shouldMAC, mac) {
		return hdr, nil, errors.New("bad frame body MAC")
	}
	rw.dec.XORKeyStream(bb, bb)

	// Truncate the buffer so it contains the content
	// without padding and the MAC.
	body.Truncate(int(fsize))
	return hdr, body, nil
}

// updateMAC reseeds the given hash with encrypted seed.
// it returns the first 16 bytes of the hash sum after seeding.
func updateMAC(mac hash.Hash, block cipher.Block, seed []byte) []byte {
	aesbuf := make([]byte, aes.BlockSize)
	block.Encrypt(aesbuf, mac.Sum(aesbuf[:0]))
	for i := range aesbuf {
		aesbuf[i] ^= seed[i]
	}
	mac.Write(aesbuf)
	return mac.Sum(nil)[:16]
}

type frameBuffer []byte

func makeFrameWriteBuffer() *frameBuffer {
	buf := make(frameBuffer, frameHeaderFullSize, frameHeaderFullSize+staticFrameSize)
	return &buf
}

// reset truncates the buffer so it contains just enough space for an
// encoded frame header. it must be called before writing payload content
// for a new frame.
func (buf *frameBuffer) reset() {
	*buf = (*buf)[:frameHeaderFullSize]
	for i := range *buf {
		(*buf)[i] = 0
	}
}

func (buf *frameBuffer) Write(s []byte) (n int, err error) {
	*buf = append(*buf, s...)
	return len(s), nil
}

func (buf *frameBuffer) pad16() {
	if padding := len(*buf) % 16; padding > 0 {
		*buf = append(*buf, zero16[:16-padding]...)
	}
}

func readInt24(b []byte) uint32 {
	return uint32(b[2]) | uint32(b[1])<<8 | uint32(b[0])<<16
}

func putInt24(s []byte, v uint32) {
	s[0] = byte(v >> 16)
	s[1] = byte(v >> 8)
	s[2] = byte(v)
}

func pad16(b *bytes.Buffer) {
	if padding := b.Len() % 16; padding > 0 {
		b.Write(zero16[:16-padding])
	}
}
