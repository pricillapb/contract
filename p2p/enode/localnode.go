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
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/p2p/netutil"
)

const (
	// IP tracker configuration
	iptrackMinStatements = 10
	iptrackWindow        = 5 * time.Minute
	iptrackContactWindow = 10 * time.Minute
)

// LocalNode produces the signed node record of a local node, i.e. a node
// run in the current process.
type LocalNode struct {
	cur atomic.Value // holds a non-nil node pointer while the record is up-to-date.
	id  ID
	key *ecdsa.PrivateKey
	db  *DB

	// everything below is protected by a lock
	mu          sync.Mutex
	seq         uint64
	entries     map[string]enr.Entry
	udpTrack    *netutil.IPTracker // predicts external UDP endpoint
	staticIP    net.IP
	staticTCP   int
	fallbackIP  net.IP
	fallbackUDP int
}

func NewLocalNode(db *DB, key *ecdsa.PrivateKey) *LocalNode {
	ln := &LocalNode{
		id:       PubkeyToIDV4(&key.PublicKey),
		db:       db,
		key:      key,
		seq:      db.LocalSeq(),
		udpTrack: netutil.NewIPTracker(iptrackWindow, iptrackContactWindow, iptrackMinStatements),
		entries:  make(map[string]enr.Entry),
	}
	ln.cur.Store((*Node)(nil))
	return ln
}

// Database returns the node database associated with the local node.
func (ln *LocalNode) Database() *DB {
	return ln.db
}

// Node returns the current version of the local node record.
func (ln *LocalNode) Node() *Node {
	n := ln.cur.Load().(*Node)
	if n != nil {
		return n
	}
	// Record was invalidated, sign a new copy.
	ln.mu.Lock()
	defer ln.mu.Unlock()
	ln.sign()
	return ln.cur.Load().(*Node)
}

// ID returns the local node ID.
func (ln *LocalNode) ID() ID {
	return ln.id
}

// Set puts the given entry into the local record, overwriting
// any existing value.
func (ln *LocalNode) Set(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.set(e)
}

func (ln *LocalNode) set(e enr.Entry) {
	val, exists := ln.entries[e.ENRKey()]
	if !exists || !reflect.DeepEqual(val, e) {
		ln.entries[e.ENRKey()] = e
		ln.cur.Store((*Node)(nil))
	}
}

// Delete removes the given entry from the local record.
func (ln *LocalNode) Delete(e enr.Entry) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.delete(e)
}

func (ln *LocalNode) delete(e enr.Entry) {
	_, exists := ln.entries[e.ENRKey()]
	if exists {
		delete(ln.entries, e.ENRKey())
		ln.cur.Store((*Node)(nil))
	}
}

func (ln *LocalNode) SetStaticIP(ip net.IP) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.staticIP = ip
	ln.updateEndpoints()
}

func (ln *LocalNode) SetFallbackIP(ip net.IP) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.fallbackIP = ip
	ln.updateEndpoints()
}

func (ln *LocalNode) SetFallbackUDP(port int) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.fallbackUDP = port
	ln.updateEndpoints()
}

func (ln *LocalNode) SetStaticTCP(port int) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.staticTCP = port
	ln.updateEndpoints()
}

// UDPEndpointStatement should be called whenever a statement about the local node's
// UDP endpoint is received. It feeds the local endpoint predictor.
func (ln *LocalNode) UDPEndpointStatement(fromaddr, endpoint *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddStatement(fromaddr.String(), endpoint.String())
	ln.updateEndpoints()
}

// UDPContact should be called whenever the local node has announced itself to another node
// via UDP. It feeds the local endpoint predictor.
func (ln *LocalNode) UDPContact(toaddr *net.UDPAddr) {
	ln.mu.Lock()
	defer ln.mu.Unlock()

	ln.udpTrack.AddContact(toaddr.String())
	ln.updateEndpoints()
}

func (ln *LocalNode) updateEndpoints() {
	// Determine the endpoints.
	newIP := ln.fallbackIP
	newUDP := ln.fallbackUDP
	if ip, port := predictAddr(ln.udpTrack); ip != nil {
		newIP = ip
		newUDP = port
	}
	if ln.staticIP != nil {
		newIP = ln.staticIP
	}

	// Update the record.
	if newIP != nil && !newIP.IsUnspecified() {
		ln.set(enr.IP(newIP))
	} else {
		ln.delete(enr.IP{})
	}
	if newUDP != 0 {
		ln.set(enr.UDP(newUDP))
	} else {
		ln.delete(enr.UDP(0))
	}
	if ln.staticTCP != 0 {
		ln.set(enr.TCP(ln.staticTCP))
	} else {
		ln.delete(enr.TCP(0))
	}
}

// predictAddr wraps IPTracker.PredictEndpoint, converting from its string-based
// endpoint representation to IP and port types.
func predictAddr(t *netutil.IPTracker) (net.IP, int) {
	ep := t.PredictEndpoint()
	if ep == "" {
		return nil, 0
	}
	ipString, portString, _ := net.SplitHostPort(ep)
	ip := net.ParseIP(ipString)
	port, _ := strconv.Atoi(portString)
	return ip, port
}

func (ln *LocalNode) sign() {
	if n := ln.cur.Load().(*Node); n != nil {
		return // no changes
	}

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
	log.Info("New local node record", "seq", ln.seq, "id", n.ID(), "ip", n.IP(), "udp", n.UDP(), "tcp", n.TCP())
}

func (ln *LocalNode) bumpSeq() {
	ln.seq++
	ln.db.storeLocalSeq(ln.seq)
}
