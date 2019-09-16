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
	"bytes"
	"context"
	"crypto/ecdsa"
	"reflect"
	"sort"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

func TestClientSyncTree(t *testing.T) {
	// tree, err := MakeTree(3, testrecords[:3], []string{"enrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"})
	// if err != nil {
	// 	t.Fatal(err)
	// }
	// tree.Sign(testkeys[0], "n")
	// for name, content := range tree.ToTXT("n") {
	// 	fmt.Printf("%q: %q,\n", name, content)
	// }

	r := mapResolver{
		"n":                            "enrtree-root=v1 e=QFT4PBCRX4XQCV3VUYJ6BTCEPU l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=3 sig=3FmXuVwpa8Y7OstZTx9PIb1mt8FrW7VpDOFv4AaGCsZ2EIHmhraWhe4NxYhQDlw5MjeFXYMbJjsPeKlHzmJREQE=",
		"QFT4PBCRX4XQCV3VUYJ6BTCEPU.n": "enrtree=N7ZW6LHUPSH4YQYAFWIG44M2OU,5VTS2SZK6TC3UZCOX3YIGNZ76I,3KRT2RWDGBGOIT4BVUPTMREO7A",
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": "enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"N7ZW6LHUPSH4YQYAFWIG44M2OU.n": "enr=-HW4QCmjL_ZHKP_UOCVv2avoj_tMgQAktxnCdJfx-HpeTOeDDi4EyKMnJLccLGlg5GyRo3wc4NUrJBmByM0bzZPHeHMIgmlkgnY0iXNlY3AyNTZrMaEDUybYcILt0ULt3FUY-tJEjOpWwm8-Bqf2EoBr17JOmKk=",
		"5VTS2SZK6TC3UZCOX3YIGNZ76I.n": "enr=-HW4QNPOHkqXzibYBvK1rwT5t15o2IkphtInmWwsLCpMWzmyZopJ09CMAfTcCqzTNlw0ByaZB_A1yQHNsGMh-SmwfnwEgmlkgnY0iXNlY3AyNTZrMaECxUT5ee0C-7zsA9FRx8yK7C8M8vkJ07tWHWUeqEZ7DBY=",
		"3KRT2RWDGBGOIT4BVUPTMREO7A.n": "enr=-HW4QLZHjM4vZXkbp-5xJoHsKSbE7W39FPC8283X-y8oHcHPTnDDlIlzL5ArvDUlHZVDPgmFASrh7cWgLOLxj4wprRkHgmlkgnY0iXNlY3AyNTZrMaEC3t2jLMhDpCDX5mbSEwDn4L3iUfyXzoO8G28XvjGRkrA=",
	}
	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})

	var (
		wantNodes = testrecords[:3]
		wantLinks = []string{"enrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"}
		wantSeq   = uint(3)
	)
	stree, err := c.SyncTree("enrtree://APFGGTFOBVE2ZNAB3CSMNNX6RRK3ODIRLP2AA5U4YFAA6MSYZUYTQ@n")
	if err != nil {
		t.Fatal("sync error:", err)
	}
	if !reflect.DeepEqual(sortByID(stree.Nodes()), wantNodes) {
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
		t.Fatalf("expected sync error %q, got %v", wantErr, err)
	}
}

type countResolver struct {
	qcount int
	child  Resolver
}

var (
	testkeys    []*ecdsa.PrivateKey
	testrecords []*enode.Node
)

func init() {
	testkeys = []*ecdsa.PrivateKey{
		hexkey("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"),
		hexkey("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a"),
		hexkey("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee"),
		hexkey("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"),
		hexkey("7018732ded552337dfbe3d6f7393b5fc2c4dff57d3420d16c91309a8bb47b51d"),
		hexkey("90725056af7cdbe7ba4ea470a5542c9b1e615b175ac18294e20bfad060f8cf5e"),
		hexkey("09ffd992a55877c2260e80eff793f229ed55983420532ea64d8473f87f1a981a"),
		hexkey("f51f6be7a7367376f5200ddc525ffb5835068b92c863ab92460cb31d107724a6"),
		hexkey("b300e6a49e8425bb3eda749ace0db408d43adcda3c5a808bf3abe86e059dee28"),
		hexkey("78e766d81cfa4e5f5d77b9804b6389b219b09967c392bbdf7f11c7df2931d313"),
	}
	testrecords = make([]*enode.Node, len(testkeys))
	for i, k := range testkeys {
		record := new(enr.Record)
		record.SetSeq(uint64(i))
		enode.SignV4(record, k)
		n, err := enode.New(enode.ValidSchemes, record)
		if err != nil {
			panic(err)
		}
		testrecords[i] = n
	}
	sortByID(testrecords)
}

func hexkey(s string) *ecdsa.PrivateKey {
	k, err := crypto.HexToECDSA(s)
	if err != nil {
		panic("invalid private key " + s)
	}
	return k
}

func newCountResolver(r Resolver) *countResolver {
	return &countResolver{child: r}
}

func (cr *countResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	cr.qcount++
	return cr.child.LookupTXT(ctx, name)
}

type mapResolver map[string]string

func (mr mapResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	if record, ok := mr[name]; ok {
		return []string{record}, nil
	}
	return nil, nil
}

func sortByID(nodes []*enode.Node) []*enode.Node {
	sort.Slice(nodes, func(i, j int) bool {
		return bytes.Compare(nodes[i].ID().Bytes(), nodes[j].ID().Bytes()) < 0
	})
	return nodes
}
