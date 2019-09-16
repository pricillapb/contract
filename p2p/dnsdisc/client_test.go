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
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/internal/testlog"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

func TestClientSyncTree(t *testing.T) {
	tree, err := MakeTree(3, testrecords[:3], []string{"enrtree://AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org"})
	if err != nil {
		t.Fatal(err)
	}
	tree.Sign(testkeys[0], "n")
	// fmt.Println(url)
	// for name, content := range tree.ToTXT("n") {
	// 	fmt.Printf("%q: %q,\n", name, content)
	// }

	r := mapResolver{
		"n":                            "enrtree-root=v1 e=TOFRR3BZDNHSY6XGP6T77KIK4Q l=JGUFMSAGI7KZYB3P7IZW4S5Y3A seq=3 sig=JW-YXz4FQP1OUbjzLF3k2zkcwqqyvKer-E3hUgf5vRpyaRT7nEUqEgTFbGS-MTDnWDR5L6d26wNZB0lLckIUQQA=",
		"TOFRR3BZDNHSY6XGP6T77KIK4Q.n": "enrtree=6ENOKQN2BOKNIFFQW56UPMFC7A,NKPAZPIMU5YADTIXWWYVICDVQI,RXN7EWFAAL6Y6XOWO3EPO4UKMM",
		"JGUFMSAGI7KZYB3P7IZW4S5Y3A.n": "enrtree-link=AM5FCQLWIZX2QFPNJAP7VUERCCRNGRHWZG3YYHIUV7BVDQ5FDPRT2@morenodes.example.org",
		"6ENOKQN2BOKNIFFQW56UPMFC7A.n": "enr=-HW4QO7fsG9CUu_f9SPwjFx8pKL7Huyu5oULkdKNg5rpZndpD-Y_QIIb9AwOx1MoclwQoPWb47EbjmeLOXJq0CnZmF8CgmlkgnY0iXNlY3AyNTZrMaED_aHP9nTJDJoZdTn-PftTCGrOZPg-18bqvsdB9_OBzIA=",
		"NKPAZPIMU5YADTIXWWYVICDVQI.n": "enr=-HW4QHihi1ZXeISPgoEfNZ1X8Y9MJnG5lEe2ahcfxLlY4GS2YJwW26w8UltTCGN64jwqdQJKccqohlzU3sS8qLIU8OIBgmlkgnY0iXNlY3AyNTZrMaEC7XwtBeeStrNXoEYa3OsFl-XTmI6pWvjrighCz_djt5A=",
		"RXN7EWFAAL6Y6XOWO3EPO4UKMM.n": "enr=-HW4QPl99oxKbLdy5R-wcLCEz4DjqXKwhMPIdGDdrmm0VVt-Qh55V6jAVKIwWpJapx7UGX6Hzf80fnTr2lFunBb7mJCAgmlkgnY0iXNlY3AyNTZrMaEDymNMrg1JrLQB2KTGtv6MVbcNEVv0AHacwUAPMljNMTg=",
	}
	c, _ := NewClient(Config{Resolver: r, Logger: testlog.Logger(t, log.LvlTrace)})
	stree, err := c.SyncTree("enrtree://APFGGTFOBVE2ZNAB3CSMNNX6RRK3ODIRLP2AA5U4YFAA6MSYZUYTQ@n")
	if err != nil {
		t.Fatal("sync error:", err)
	}
	if !reflect.DeepEqual(stree, tree) {
		t.Error("incomplete tree synced")
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
