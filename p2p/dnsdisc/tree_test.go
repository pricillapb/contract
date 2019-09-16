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
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

func TestParseRoot(t *testing.T) {
	tests := []struct {
		input string
		e     rootEntry
		err   error
	}{
		{
			input: "enrtree-root=v1 e=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtw=",
			err:   entryError{"root", errSyntax},
		},
		{
			input: "enrtree-root=v1 e=TO4Q75OQ2N7DX4EOOR7X66A6OM l=TO4Q75OQ2N7DX4EOOR7X66A6OM seq=3 sig=N-YY6UB9xD0hFx1Gmnt7v0RfSxch5tKyry2SRDoLx7B4GfPXagwLxQqyf7gAMvApFn_ORwZQekMWa_pXrcGCtw=",
			err:   entryError{"root", errInvalidSig},
		},
	}
	for i, test := range tests {
		e, err := parseRoot(test.input)
		if !reflect.DeepEqual(e, test.e) {
			t.Errorf("test %d: wrong entry %s, want %s", i, spew.Sdump(e), spew.Sdump(test.e))
		}
		if err != test.err {
			t.Errorf("test %d: wrong error %q, want %q", i, err, test.err)
		}
	}
}

func TestParseEntry(t *testing.T) {
	tests := []struct {
		input string
		e     entry
		err   error
	}{
		// Subtrees:
		{
			input: "enrtree=1,2",
			err:   entryError{"subtree", errInvalidChild},
		},
		{
			input: "enrtree=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			err:   entryError{"subtree", errInvalidChild},
		},
		{
			input: "enrtree=",
			e:     &subtreeEntry{},
		},
		{
			input: "enrtree=AAAAAAAAAAAAAAAAAA",
			e:     &subtreeEntry{[]string{"AAAAAAAAAAAAAAAAAA"}},
		},
		{
			input: "enrtree=AAAAAAAAAAAAAAAAAA,BBBBBBBBBBBBBBBBBB",
			e:     &subtreeEntry{[]string{"AAAAAAAAAAAAAAAAAA", "BBBBBBBBBBBBBBBBBB"}},
		},
		// Links
		{
			input: "enrtree-link=AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@nodes.example.org",
			e:     &linkEntry{"nodes.example.org", &testkeys[2].PublicKey},
		},
		{
			input: "enrtree-link=nodes.example.org",
			err:   entryError{"link", errNoPubkey},
		},
		{
			input: "enrtree-link=AP62DT7WOTEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57@nodes.example.org",
			err:   entryError{"link", errBadPubkey},
		},
		{
			input: "enrtree-link=AP62DT7WONEQZGQZOU474PP3KMEGVTTE7A7NPRXKX3DUD57TQHGIA@nodes.example.org",
			err:   entryError{"link", errBadPubkey},
		},
		// ENRs
		{
			input: "enr=-HW4QLZHjM4vZXkbp-5xJoHsKSbE7W39FPC8283X-y8oHcHPTnDDlIlzL5ArvDUlHZVDPgmFASrh7cWgLOLxj4wprRkHgmlkgnY0iXNlY3AyNTZrMaEC3t2jLMhDpCDX5mbSEwDn4L3iUfyXzoO8G28XvjGRkrA=",
			e:     &enrEntry{record: testrecords[7].Record()},
		},
		{
			input: "enr=-HW4QLZHjM4vZXkbp-5xJoHsKSbE7W39FPC8283X-y8oHcHPTnDDlIlzL5ArvDUlHZVDPgmFASrh7cWgLOLxj4wprRkHgmlkgnY0iXNlY3AyNTZrMaEC3t2jLMhDpCDX5mbSEwDn4L3iUfyXzoO8G28XvjGRkrAg=",
			err:   entryError{"enr", errInvalidENR},
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

	tree, err := MakeTree(2, nodes, nil)
	if err != nil {
		t.Fatal(err)
	}
	txt := tree.ToTXT("")
	if len(txt) < len(nodes)+1 {
		t.Fatal("too few TXT records in output")
	}
}
