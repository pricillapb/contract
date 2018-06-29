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

package discover

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// The "null" ENR identity scheme. This scheme stores the node
// ID in the record without any signature.
type nullID struct{}

func (nullID) Verify(r *enr.Record, sig []byte) error {
	return nil
}

func (nullID) NodeAddr(r *enr.Record) []byte {
	var id enode.ID
	r.Load(enr.WithEntry("nulladdr", &id))
	return id[:]
}

func init() {
	enr.RegisterIdentityScheme("null", nullID{})
}

func setID(r *enode.Node, id enode.ID) {
	r.Set(enr.ID("null"))
	r.Set(enr.WithEntry("nulladdr", id))
	if err := r.SetSig("null", nil); err != nil {
		panic(err)
	}
}

func newTestTable(t transport) (*Table, *enode.DB) {
	var n enode.Node
	n.Set(enr.IP{0, 0, 0, 0})
	setID(&n, enode.ID{})
	db, _ := enode.NewDB("", enode.ID{})
	tab, _ := newTable(t, &n, db, nil)
	return tab, db
}

// nodeAtDistance creates a node for which enode.LogDist(base, n.id) == ld.
func nodeAtDistance(base enode.ID, ld int) *Node {
	n := new(enode.Node)
	setID(n, idAtDistance(base, ld))
	n.Set(enr.IP{byte(ld), 0, 2, byte(ld)})
	return convertNode(n)
}

// idAtDistance returns a random hash such that enode.LogDist(a, b) == n
func idAtDistance(a enode.ID, n int) (b enode.ID) {
	if n == 0 {
		return a
	}
	// flip bit at position n, fill the rest with random bits
	b = a
	pos := len(a) - n/8 - 1
	bit := byte(0x01) << (byte(n%8) - 1)
	if bit == 0 {
		pos++
		bit = 0x80
	}
	b[pos] = a[pos]&^bit | ^a[pos]&bit // TODO: randomize end bits
	for i := pos + 1; i < len(a); i++ {
		b[i] = byte(rand.Intn(255))
	}
	return b
}

// fillBucket inserts nodes into the given bucket until it is full.
func fillBucket(tab *Table, n *Node) (last *Node) {
	ld := enode.LogDist(tab.self.id, n.id)
	b := tab.bucket(n.id)
	for len(b.entries) < bucketSize {
		b.entries = append(b.entries, nodeAtDistance(tab.self.id, ld))
	}
	return b.entries[bucketSize-1]
}

type pingRecorder struct {
	mu           sync.Mutex
	dead, pinged map[enode.ID]bool
}

func newPingRecorder() *pingRecorder {
	return &pingRecorder{
		dead:   make(map[enode.ID]bool),
		pinged: make(map[enode.ID]bool),
	}
}

func (t *pingRecorder) findnode(toid enode.ID, toaddr *net.UDPAddr, target encPubkey) ([]*Node, error) {
	return nil, nil
}

func (t *pingRecorder) waitping(from enode.ID) error {
	return nil // remote always pings
}

func (t *pingRecorder) ping(toid enode.ID, toaddr *net.UDPAddr) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.pinged[toid] = true
	if t.dead[toid] {
		return errTimeout
	} else {
		return nil
	}
}

func (t *pingRecorder) close() {}

func hasDuplicates(slice []*Node) bool {
	seen := make(map[enode.ID]bool)
	for i, e := range slice {
		if e == nil {
			panic(fmt.Sprintf("nil *Node at %d", i))
		}
		if seen[e.id] {
			return true
		}
		seen[e.id] = true
	}
	return false
}

func contains(ns []*Node, id enode.ID) bool {
	for _, n := range ns {
		if n.id == id {
			return true
		}
	}
	return false
}

func sortedByDistanceTo(distbase enode.ID, slice []*Node) bool {
	var last enode.ID
	for i, e := range slice {
		if i > 0 && enode.DistCmp(distbase, e.id, last) < 0 {
			return false
		}
		last = e.id
	}
	return true
}

func hexEncPubkey(h string) (ret encPubkey) {
	b, err := hex.DecodeString(h)
	if err != nil {
		panic(err)
	}
	if len(b) != len(ret) {
		panic("invalid length")
	}
	copy(ret[:], b)
	return ret
}

func hexPubkey(h string) *ecdsa.PublicKey {
	k, err := decodePubkey(hexEncPubkey(h))
	if err != nil {
		panic(err)
	}
	return k
}
