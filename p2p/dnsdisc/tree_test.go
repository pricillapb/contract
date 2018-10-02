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
)

var testkeys = []*ecdsa.PrivateKey{
	hexkey("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291"),
	hexkey("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a"),
	hexkey("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee"),
	hexkey("45a915e4d060149eb4365960e6a7a45f334393093061116b197e3240065ff2d8"),
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
