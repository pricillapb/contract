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

package discover

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

func TestResolveENRv4(t *testing.T) {
	var (
		tab0, ln0 = startTestNode(t, nil)
		tab1, _   = startTestNode(t, ln0.Node())
		tab2, ln2 = startTestNode(t, ln0.Node())
	)
	defer tab0.Close()
	defer tab1.Close()
	defer tab2.Close()

	// Resolve node 2. The result should be node 2's current record.
	if n2 := tab1.Resolve(ln2.Node()); !reflect.DeepEqual(n2.Record(), ln2.Node().Record()) {
		t.Errorf("wrong resolve result for node 2: %s", spew.Sdump(n2.Record()))
	}
	// Add a key to node 2's record and resolve again. The result should include the new key.
	ln2.Set(enr.WithEntry("x", uint(3)))
	if n2 := tab1.Resolve(ln2.Node()); !reflect.DeepEqual(n2.Record(), ln2.Node().Record()) {
		t.Errorf("wrong resolve result for node 2: %s", spew.Sdump(n2.Record()))
	}
}

func TestLookupENRv4(t *testing.T) {
	var (
		tab0, ln0 = startTestNode(t, nil)
		tab1, _   = startTestNode(t, ln0.Node())
		tab2, ln2 = startTestNode(t, ln0.Node())
	)
	defer tab0.Close()
	defer tab1.Close()
	defer tab2.Close()

	// Wait for node 2 to appear in the bootnode table, otherwise we won't find it.
	for i := 0; i < 30; i++ {
		if tab0.findInBucket(ln2.ID()) != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Lookup node 2. It should return the up-to-date node record among the results.
	found := find(tab1.lookup(encodePubkey(ln2.Node().Pubkey()), true), ln2.ID())
	if found == nil {
		t.Fatalf("node 2 not found during lookup")
	}
	if !reflect.DeepEqual(found.Record(), ln2.Node().Record()) {
		t.Errorf("wrong lookup result for node 2: %s", spew.Sdump(found.Record()))
	}
}

func startTestNode(t *testing.T, bootnode *enode.Node) (*Table, *enode.LocalNode) {
	var (
		db, _     = enode.OpenDB("")
		key, _    = crypto.GenerateKey()
		ln        = enode.NewLocalNode(db, key)
		bootnodes []*enode.Node
	)
	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IP{127, 0, 0, 1}})
	if err != nil {
		t.Fatalf("can't listen: %v", err)
	}
	ln.SetStaticIP(net.IP{127, 0, 0, 1})
	ln.SetFallbackUDP(conn.LocalAddr().(*net.UDPAddr).Port)
	if bootnode != nil {
		bootnodes = append(bootnodes, bootnode)
	}
	tab, err := ListenUDP(conn, ln, Config{
		PrivateKey: key,
		Bootnodes:  bootnodes,
	})
	if err != nil {
		t.Fatalf("can't start table: %v", err)
	}
	return tab, ln
}
