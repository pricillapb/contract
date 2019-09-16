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

package dnsdisc

import (
	"context"
	"crypto/ecdsa"
	"math/rand"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

const (
	signingKeySeed = 0x111111
	nodesSeed1     = 0x2945237
	nodesSeed2     = 0x4567299
)

func TestClientSyncTree(t *testing.T) {
	r := mapResolver{
		"3CA2MBMUQ55ZCT74YEEQLANJDI.n": "enr=-HW4QAggRauloj2SDLtIHN1XBkvhFZ1vtf1raYQp9TBW2RD5EEawDzbtSmlXUfnaHcvwOizhVYLtr7e6vw7NAf6mTuoCgmlkgnY0iXNlY3AyNTZrMaECjrXI8TLNXU0f8cthpAMxEshUyQlK-AM0PW2wfrnacNI=",
		"53HBTPGGZ4I76UEPCNQGZWIPTQ.n": "enr=-HW4QOFzoVLaFJnNhbgMoDXPnOvcdVuj7pDpqRvh6BRDO68aVi5ZcjB3vzQRZH2IcLBGHzo8uUN3snqmgTiE56CH3AMBgmlkgnY0iXNlY3AyNTZrMaECC2_24YYkYHEgdzxlSNKQEnHhuNAbNlMlWJxrJxbAFvA=",
		"BG7SVUBUAJ3UAWD2ATEBLMRNEE.n": "enrtree=53HBTPGGZ4I76UEPCNQGZWIPTQ,3CA2MBMUQ55ZCT74YEEQLANJDI,HNHR6UTVZF5TJKK3FV27ZI76P4",
		"HNHR6UTVZF5TJKK3FV27ZI76P4.n": "enr=-HW4QLAYqmrwllBEnzWWs7I5Ev2IAs7x_dZlbYdRdMUx5EyKHDXp7AV5CkuPGUPdvbv1_Ms1CPfhcGCvSElSosZmyoqAgmlkgnY0iXNlY3AyNTZrMaECriawHKWdDRk2xeZkrOXBQ0dfMFLHY4eENZwdufn1S1o=",
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": "enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"n":                            "enrtree-root=v1 e=BG7SVUBUAJ3UAWD2ATEBLMRNEE l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=1 sig=gacuU0nTy9duIdu1IFDyF5Lv9CFHqHiNcj91n0frw70tZo3tZZsCVkE3j1ILYyVOHRLWGBmawo_SEkThZ9PgcQE=",
	}
	var (
		wantNodes = testNodes(0x29452, 3)
		wantLinks = []string{"enrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"}
		wantSeq   = uint(1)
	)
	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	stree, err := c.SyncTree("enrtree://AKPYQIUQIL7PSIACI32J7FGZW56E5FKHEFCCOFHILBIMW3M6LWXS2@n")
	if err != nil {
		t.Fatal("sync error:", err)
	}
	if !reflect.DeepEqual(sortByID(stree.Nodes()), sortByID(wantNodes)) {
		t.Errorf("wrong nodes in synced tree:\nhave %v\nwant %v", spew.Sdump(stree.Nodes()), spew.Sdump(wantNodes))
	}
	if !reflect.DeepEqual(stree.Links(), wantLinks) {
		t.Errorf("wrong links in synced tree: %v", stree.Links())
	}
	if stree.Seq() != wantSeq {
		t.Errorf("synced tree has wrong seq: %d", stree.Seq())
	}
}

// In this test, syncing the tree fails because it contains an invalid ENR entry.
func TestClientSyncTreeBadNode(t *testing.T) {
	r := mapResolver{
		"n":                            "enrtree-root=v1 e=ZFJZDQKSOMJRYYQSZKJZC54HCF l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=3 sig=WEy8JTZ2dHmXM2qeBZ7D2ECK7SGbnurl1ge_S_5GQBAqnADk0gLTcg8Lm5QNqLHZjJKGAb443p996idlMcBqEQA=",
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": "enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"ZFJZDQKSOMJRYYQSZKJZC54HCF.n": "enr=gggggggggggggg=",
	}

	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	_, err := c.SyncTree("enrtree://APFGGTFOBVE2ZNAB3CSMNNX6RRK3ODIRLP2AA5U4YFAA6MSYZUYTQ@n")
	wantErr := nameError{name: "ZFJZDQKSOMJRYYQSZKJZC54HCF.n", err: entryError{typ: "enr", err: errInvalidENR}}
	if err != wantErr {
		t.Fatalf("expected sync error %q, got %q", wantErr, err)
	}
}

// This test checks that RandomNode hits all entries.
func TestClientRandomNode(t *testing.T) {
	nodes := testNodes(nodesSeed1, 30)
	tree, url := makeTestTree("n", nodes, []string{})
	r := mapResolver(tree.ToTXT("n"))
	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	if err := c.AddTree(url); err != nil {
		t.Fatal(err)
	}

	checkRandomNode(t, c, nodes)
}

// This test checks that RandomNode traverses linked trees as well as explicitly added trees.
func TestClientRandomNodeLinks(t *testing.T) {
	nodes := testNodes(nodesSeed1, 40)
	tree1, url1 := makeTestTree("t1", nodes[:10], []string{})
	tree2, url2 := makeTestTree("t2", nodes[10:], []string{url1})
	cfg := Config{
		Resolver: newMapResolver(tree1.ToTXT("t1"), tree2.ToTXT("t2")),
		Logger:   testlog.Logger(t, log.LvlTrace),
	}
	c, _ := NewClient(cfg)
	if err := c.AddTree(url2); err != nil {
		t.Fatal(err)
	}

	checkRandomNode(t, c, nodes)
}

func checkRandomNode(t *testing.T, c *Client, wantNodes []*enode.Node) {
	t.Helper()

	var (
		seen     = make(map[enode.ID]*enode.Node)
		calls    = 0
		maxCalls = 200
		ctx      = context.Background()
	)
	for len(seen) < len(wantNodes) && calls < maxCalls {
		calls++
		n := c.RandomNode(ctx)
		seen[n.ID()] = n
	}
	if calls >= maxCalls {
		t.Fatalf("too many calls: %d, want at most %d", calls, maxCalls)
	}
	for _, n := range wantNodes {
		if seen[n.ID()] == nil {
			t.Errorf("RandomNode didn't discover node %v", n.ID())
		}
	}
}

func makeTestTree(domain string, nodes []*enode.Node, links []string) (*Tree, string) {
	tree, err := MakeTree(1, nodes, links)
	if err != nil {
		panic(err)
	}
	url, err := tree.Sign(testKey(signingKeySeed), domain)
	if err != nil {
		panic(err)
	}
	return tree, url
}

// testKeys creates deterministic private keys for testing.
func testKeys(seed int64, n int) []*ecdsa.PrivateKey {
	rand := rand.New(rand.NewSource(seed))
	keys := make([]*ecdsa.PrivateKey, n)
	for i := 0; i < n; i++ {
		key, err := ecdsa.GenerateKey(crypto.S256(), rand)
		if err != nil {
			panic("can't generate key: " + err.Error())
		}
		keys[i] = key
	}
	return keys
}

func testKey(seed int64) *ecdsa.PrivateKey {
	return testKeys(seed, 1)[0]
}

func testNodes(seed int64, n int) []*enode.Node {
	keys := testKeys(seed, n)
	nodes := make([]*enode.Node, n)
	for i, key := range keys {
		record := new(enr.Record)
		record.SetSeq(uint64(i))
		enode.SignV4(record, key)
		n, err := enode.New(enode.ValidSchemes, record)
		if err != nil {
			panic(err)
		}
		nodes[i] = n
	}
	return nodes
}

func testNode(seed int64) *enode.Node {
	return testNodes(seed, 1)[0]
}

type mapResolver map[string]string

func newMapResolver(maps ...map[string]string) mapResolver {
	mr := make(mapResolver)
	for _, m := range maps {
		for k, v := range m {
			mr[k] = v
		}
	}
	return mr
}

func (mr mapResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if record, ok := mr[name]; ok {
		return []string{record}, nil
	}
	return nil, nil
}
