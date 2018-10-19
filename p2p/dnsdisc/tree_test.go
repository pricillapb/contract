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
	"crypto/ecdsa"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

var testkeys = []*ecdsa.PrivateKey{
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

func hexkey(s string) *ecdsa.PrivateKey {
	k, err := crypto.HexToECDSA(s)
	if err != nil {
		panic("invalid private key " + s)
	}
	return k
}

func TestParseEntry(t *testing.T) {
	tests := []struct {
		input string
		e     interface{}
		err   error
	}{
		// Roots:
		{input: "enrtree-root=v1 hash=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtw="},
		// Subtrees:
		{input: "enrtree=", err: entryError{"subtree", errInvalidChild}},
		{input: "enrtree=1,2", err: entryError{"subtree", errInvalidChild}},
		{input: "enrtree=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", err: entryError{"subtree", errInvalidChild}},
		{input: "enrtree=AAAAAAAAAAAAAAAAAA", e: subtreeEntry{[]string{"AAAAAAAAAAAAAAAAAA"}}},
		{input: "enrtree=AAAAAAAAAAAAAAAAAA,BBBBBBBBBBBBBBBBBB", e: subtreeEntry{[]string{"AAAAAAAAAAAAAAAAAA", "BBBBBBBBBBBBBBBBBB"}}},
		// Links
		{input: "enrtree-link=AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@nodes.example.org", e: linkEntry{"nodes.example.org", &testkeys[2].PublicKey}},
		{input: "enrtree-link=nodes.example.org", err: entryError{"link", errNoPubkey}},
		{input: "enrtree-link=AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57@nodes.example.org", err: entryError{"link", errBadPubkey}},
		{input: "enrtree-link=AP62DT7WONEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@nodes.example.org", err: entryError{"link", errBadPubkey}},
		// ENRs
		{
			input: "enr=-H24QI0fqW39CMBZjJvV-EJZKyBYIoqvh69kfkF4X8DsJuXOZC6emn53SrrZD8P4v9Wp7NxgDYwtEUs3zQkxesaGc6UBgmlkgnY0gmlwhMsAcQGJc2VjcDI1NmsxoQPKY0yuDUmstAHYpMa2_oxVtw0RW_QAdpzBQA8yWM0xOA==",
		},
		// Invalid:
		{input: "", err: errUnknownEntry},
		{input: "foo", err: errUnknownEntry},
		{input: "enrtree", err: errUnknownEntry},
		{input: "enrtree-x=", err: errUnknownEntry},
	}
	for i, test := range tests {
		e, err := parseEntry(test.input)
		if !reflect.DeepEqual(e, test.e) {
			t.Errorf("test %d: wrong entry %s, want %s", i, spew.Sdump(e), spew.Sdump(test.e))
		}
		if err != test.err {
			t.Errorf("test %d: wrong error %q, want %q", i, err, test.err)
		}
	}
}

func TestMakeTree(t *testing.T) {
	nodes := make([]*enode.Node, len(testkeys))
	for i := range nodes {
		var r enr.Record
		enode.SignV4(&r, testkeys[i])
		n, _ := enode.New(enode.ValidSchemes, &r)
		nodes[i] = n
	}

	tree, err := MakeTree(nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	spew.Dump(tree.ToTXT(""))
}
