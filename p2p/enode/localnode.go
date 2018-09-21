// Copyright 2018 The go-ethereum Authors
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

package enode

import (
	"crypto/ecdsa"
	"fmt"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

// LocalNode produces the signed node record of a local node, i.e. a node
// run in the current process.
type LocalNode struct {
	cur atomic.Value
	key *ecdsa.PrivateKey
	db  *DB

	mu       sync.Mutex
	seq      uint64
	entries  map[string]enr.Entry
	udpTrack *netutil.IPTracker
}

func NewLocalNode(db *DB, key *ecdsa.PrivateKey) *LocalNode {
	ln := &LocalNode{
		db:       db,
		key:      key,
		seq:      db.LocalSeq(),
		udpTrack: netutil.NewIPTracker(20*time.Second, 60*time.Second, 0),
		entries:  make(map[string]enr.Entry),
	}
	ln.sign()
	return ln
}

// Database returns the node database associated with the local node.
func (ln *LocalNode) Database() *DB {
	return ln.db
}

// Node returns the current version of the local node record.
func (ln *LocalNode) Node() *Node {
	return ln.cur.Load().(*Node)
}

// ID returns the local node ID.
func (ln *LocalNode) ID() ID {
	return ln.cur.Load().(*Node).ID()
}

// Set puts the given entry into the local record, overwriting
// any existing value.
func (ln *LocalNode) Set(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.set(e)
	ln.sign()
}

func (ln *LocalNode) set(e enr.Entry) {
	ln.entries[e.ENRKey()] = e
}

// Delete removes the given entry from the local record.
func (ln *LocalNode) Delete(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.delete(e)
	ln.sign()
}

func (ln *LocalNode) delete(e enr.Entry) {
	delete(ln.entries, e.ENRKey())
}

// AddEndpointStatementUDP should be called whenever a statement about the local node's
// UDP endpoint is received. It feeds the local endpoint predictor.
func (ln *LocalNode) AddEndpointStatementUDP(from, local *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddStatement(from.String(), local.String())
	ln.updateIP()
}

// AddContactUDP should be called whenever the local node is in contact
// with another node via UDP. It feeds the local endpoint predictor.
func (ln *LocalNode) AddContactUDP(addr *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddContact(addr.String())
	ln.updateIP()
}

func (ln *LocalNode) updateIP() {
	cur := ln.Node()
	curep := &net.UDPAddr{IP: cur.IP(), Port: cur.UDP()}
	newep := ln.udpTrack.PredictEndpoint()
	if newep == curep.String() {
		return // no changes
	}
	if newep == "" {
		ln.delete(enr.IP{})
		ln.delete(enr.UDP(0))
	} else {
		ipString, portString, _ := net.SplitHostPort(newep)
		ip := net.ParseIP(ipString)
		port, _ := strconv.Atoi(portString)
		ln.set(enr.IP(ip))
		ln.set(enr.UDP(port))
	}
	ln.sign()
}

func (ln *LocalNode) sign() {
	var r enr.Record
	for _, e := range ln.entries {
		r.Set(e)
	}
	ln.bumpSeq()
	r.SetSeq(ln.seq)
	if err := SignV4(&r, ln.key); err != nil {
		panic(fmt.Errorf("enode: can't sign record: %v", err))
	}
	n, err := New(ValidSchemes, &r)
	if err != nil {
		panic(fmt.Errorf("enode: can't verify local record: %v", err))
	}
	ln.cur.Store(n)
	log.Info("New local node record", "seq", ln.seq, "ip", n.IP(), "udp", n.UDP())
}

func (ln *LocalNode) bumpSeq() {
	ln.seq++
	ln.db.storeLocalSeq(ln.seq)
}
