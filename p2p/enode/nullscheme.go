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
	"github.com/ethereum/go-ethereum/p2p/enr"
)

var ValidSchemesForTesting = enr.SchemeMap{
	"v4":   enr.V4ID{},
	"null": nullID{},
}

// The "null" ENR identity scheme. This scheme stores the node
// ID in the record without any signature.
type nullID struct{}

func (nullID) Verify(r *enr.Record, sig []byte) error {
	return nil
}

func (nullID) NodeAddr(r *enr.Record) []byte {
	var id ID
	r.Load(enr.WithEntry("nulladdr", &id))
	return id[:]
}

func setID(r *Node, id ID) {
	r.Set(enr.ID("null"))
	r.Set(enr.WithEntry("nulladdr", id))
	if err := r.SetSig(nullID{}, nil); err != nil {
		panic(err)
	}
}
