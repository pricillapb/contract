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

	"github.com/ethereum/go-ethereum/p2p/enr"
)

// List of known secure identity schemes.
var ValidSchemes = enr.SchemeMap{
	"v4": enr.V4ID{},
}

var ValidSchemesForTesting = enr.SchemeMap{
	"v4":   enr.V4ID{},
	"null": NullID{},
}

type v4CompatID struct {
	enr.V4ID
}

func (v4CompatID) Verify(r *enr.Record, sig []byte) error {
	var pubkey enr.Secp256k1
	return r.Load(&pubkey)
}

func signV4Compat(r *enr.Record, pubkey *ecdsa.PublicKey) {
	r.Set((*enr.Secp256k1)(pubkey))
	if err := r.SetSig(v4CompatID{}, []byte{}); err != nil {
		panic(err)
	}
}

// NullID is the "null" ENR identity scheme. This scheme stores the node
// ID in the record without any signature.
type NullID struct{}

func (NullID) Verify(r *enr.Record, sig []byte) error {
	return nil
}

func (NullID) NodeAddr(r *enr.Record) []byte {
	var id ID
	r.Load(enr.WithEntry("nulladdr", &id))
	return id[:]
}

func SignNull(r *enr.Record, id ID) *Node {
	r.Set(enr.ID("null"))
	r.Set(enr.WithEntry("nulladdr", id))
	if err := r.SetSig(NullID{}, []byte{}); err != nil {
		panic(err)
	}
	return &Node{r: *r, id: id}
}
