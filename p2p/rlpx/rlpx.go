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

// Package rlpx implements the RLPx secure transport protocol.
//
// RLPx multiplexes packet streams over an authenticated and encrypted
// network connection.
//
// The protocol specification lives at https://github.com/ethereum/devp2p.
//
// Protocols
//
// RLPx transports packet streams for multiple protocols on the same
// connection, ensuring that available bandwidth is fairly distributed
// among them. Negotiation of protocol identifiers is not part of the
// transport layer and is typically done by sending messages with
// protocol 0.
package rlpx

import (
	"crypto/ecdsa"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

// A Config structure is used to configure an RLPx client or server
// connection. After one has been passed to any function in package
// rlpx, it must not be modified. A Config may be reused; the rlpx
// package will also not modify it.
type Config struct {
	// Key is the private key of the server. The key must use the
	// secp256k1 curve, other curves are not supported.
	// This field is required for both client and server connections.
	Key *ecdsa.PrivateKey
}

// A Conn represents an rlpx connection.
type Conn struct {
	// readonly fields
	cfg       *Config
	isServer  bool
	fd        net.Conn
	handshake sync.Once

	mu       sync.Mutex
	rw       *frameRW // set after handshake
	remoteID *ecdsa.PublicKey
	proto    map[uint16]*Protocol
	readErr  error

	wmu sync.Mutex // excludes writes on rw
}

// Client returns a new client side RLPx connection using fd as the
// underlying transport. The public key of the remote end must be
// known in advance.
//
// config must not be nil and must contain a
// valid private key.
func Client(fd net.Conn, remotePubkey *ecdsa.PublicKey, config *Config) *Conn {
	return &Conn{
		fd:       fd,
		cfg:      config,
		remoteID: remotePubkey,
		proto:    make(map[uint16]*Protocol),
	}
}

// Server returns a new server side RLPx connection using fd as the
// underlying transport. The configuration config must be non-nil and
// must contain a valid private key
func Server(fd net.Conn, config *Config) *Conn {
	return &Conn{
		fd:       fd,
		cfg:      config,
		isServer: true,
		proto:    make(map[uint16]*Protocol),
	}
}

// Handshake runs the client or server handshake protocol if it has
// not yet been run. Most uses of this package need not call Handshake
// explicitly: the first Read or Write will call it automatically.
func (c *Conn) Handshake() (err error) {
	// TODO: check cfg.Key curve, maybe panic earlier
	c.handshake.Do(func() {
		var sec secrets
		if c.isServer {
			sec, err = receiverEncHandshake(c.fd, c.cfg.Key, nil)
		} else {
			sec, err = initiatorEncHandshake(c.fd, c.cfg.Key, c.remoteID, nil)
		}
		if err != nil {
			return
		}
		c.rw = newFrameRW(c.fd, sec)
		c.remoteID = sec.RemoteID
		go readLoop(c)
	})
	if err == nil && c.rw == nil {
		return errors.New("handshake failed")
	}
	return err
}

// LocalAddr returns the local network address of the underlying net.Conn.
func (c *Conn) LocalAddr() net.Addr {
	return c.fd.LocalAddr()
}

// RemoteAddr returns the remote network address of the underlying net.Conn.
func (c *Conn) RemoteAddr() net.Addr {
	return c.fd.RemoteAddr()
}

// RemoteID returns the public key of the remote end.
// If the remote identity is not yet known, it returns nil.
func (c *Conn) RemoteID() *ecdsa.PublicKey {
	c.mu.Lock()
	id := c.remoteID
	c.mu.Unlock()
	return id
}

// Close closes the connection.
func (c *Conn) Close() error {
	// TODO: shut down reader/wr
	return c.fd.Close()
}

// Protocol returns a handle for the given protocol id.
// It can be called at most once for any given id,
// subsequent call with the same id will panic.
func (c *Conn) Protocol(id uint16) *Protocol {
	p := c.getProtocol(id)
	close(p.claimSignal) // panics when claimed twice
	return p
}

// waits until the given protocol is claimed by a call to Protocol.
func (c *Conn) waitForProtocol(id uint16) *Protocol {
	p := c.getProtocol(id)
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	select {
	case <-timeout.C:
		return nil
	case <-p.claimSignal:
		return p
	}
}

func (c *Conn) getProtocol(id uint16) *Protocol {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.proto[id] == nil {
		c.proto[id] = newProtocol(c, id)
	}
	return c.proto[id]
}

// Protocol is a handle for the given protocol.
// ...
type Protocol struct {
	c           *Conn
	claimed     bool
	id          uint16
	claimSignal chan struct{}

	// for writing
	wmu          sync.Mutex
	contextidSeq uint16

	// for reading
	rmu             sync.Mutex
	xfers           map[uint16]*chunkedReader
	newPacket       chan packet
	readCloseSignal chan struct{}
	readErr         error // ok to read after readCloseSignal is closed
}

type packet struct {
	len uint32
	r   io.Reader
}

func newProtocol(c *Conn, id uint16) *Protocol {
	return &Protocol{
		c:               c,
		id:              id,
		xfers:           make(map[uint16]*chunkedReader),
		claimSignal:     make(chan struct{}),
		readCloseSignal: make(chan struct{}),
		newPacket:       make(chan packet),
	}
}

func (p *Protocol) readClose(err error) {
	p.readErr = err
	close(p.readCloseSignal)
}

// ReadHeader waits for a packet to appear. The content of the packet
// can be read from r as it is received. More packets can be read
// immediately, r does not need to be consumed before the next call.
func (p *Protocol) ReadPacket() (len uint32, r io.Reader, err error) {
	if err := p.c.Handshake(); err != nil {
		return 0, nil, err
	}
	select {
	case <-p.readCloseSignal:
		return 0, nil, p.readErr
	case pkt := <-p.newPacket:
		return pkt.len, pkt.r, nil
	}
}

// SendPacket sends len bytes from the payload reader on the connection.
func (p *Protocol) SendPacket(len uint32, payload io.Reader) error {
	// Grab the protocol lock to ensure that messages in the
	// protocol are sent one at a time.
	// TODO: remove
	if err := p.c.Handshake(); err != nil {
		return err
	}
	p.wmu.Lock()
	defer p.wmu.Unlock()
	if len <= staticFrameSize { // or old RLPx version
		return p.c.sendFrame(regularHeader{p.id, 0}, payload, len)
	}
	return p.sendChunked(len, payload)
}

// returns the next context ID for a chunked transfer.
// never returns 0, which is reserved for single-frame transfers.
func (p *Protocol) nextContextID() uint16 {
	p.contextidSeq++
	return p.contextidSeq
}

func (p *Protocol) sendChunked(totalsize uint32, body io.Reader) error {
	contextid := p.nextContextID()
	size := totalsize
	initial := true
	for seq := uint16(0); size > 0; seq++ {
		fsize := staticFrameSize
		if totalsize < fsize {
			fsize = totalsize
		}
		size -= fsize
		var header interface{}
		if initial {
			header = chunkStartHeader{p.id, contextid, totalsize}
			initial = false
		} else {
			header = regularHeader{p.id, contextid}
		}
		if err := p.c.sendFrame(header, body, fsize); err != nil {
			return err
		}
	}
	return nil
}

func (c *Conn) sendFrame(header interface{}, body io.Reader, size uint32) error {
	c.wmu.Lock()
	defer c.wmu.Unlock()
	return c.rw.sendFrame(header, body, size)
}
