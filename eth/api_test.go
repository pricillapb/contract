// Copyright 2016 The go-ethereum Authors
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

package eth

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/ethdb"
)

var dumper = spew.ConfigState{Indent: "    "}

func TestStorageRangeAt(t *testing.T) {
	// Create a state where account 0x010000... has a few storage entries.
	var (
		db, _    = ethdb.NewMemDatabase()
		state, _ = state.New(common.Hash{}, db)
		addr     = common.Address{0x01}
		storage  = storageMap{
			{
				Key: common.Hash{0x01}, Value: common.Hash{0x01},
				HashedKey: common.HexToHash("48078cfed56339ea54962e72c37c7f588fc4f8e5bc173827ba75cb10a63a96a5"),
			},
			{
				Key: common.Hash{0x02}, Value: common.Hash{0x02},
				HashedKey: common.HexToHash("340dd630ad21bf010b4e676dbfa9ba9a02175262d1fa356232cfde6cb5b47ef2"),
			},
			{
				Key: common.Hash{0x03}, Value: common.Hash{0x03},
				HashedKey: common.HexToHash("5723d2c3a83af9b735e3b7f21531e5623d183a9095a56604ead41f3582fdfb75"),
			},
			{
				Key: common.Hash{0x04}, Value: common.Hash{0x04},
				HashedKey: common.HexToHash("426fcb404ab2d5d8e61a3d918108006bbb0a9be65e92235bb10eefbdb6dcd053"),
			},
		}
	)
	for _, entry := range storage {
		state.SetState(addr, entry.Key, entry.Value)
	}

	// Check a few combinations of limit and start/end.
	tests := []struct {
		start, end []byte
		limit      int
		want       StorageRangeResult
	}{
		{start: []byte{}, end: []byte{}, limit: 0, want: StorageRangeResult{storageMap{}, false}},
		{start: []byte{}, end: []byte{}, limit: 100, want: StorageRangeResult{storage, true}},
		{start: []byte{}, end: []byte{}, limit: 2, want: StorageRangeResult{storage[:2], false}},
		{start: []byte{0x00}, end: []byte{0xff}, limit: 4, want: StorageRangeResult{storage, true}},
		{start: []byte{0x02}, end: []byte{0xff}, limit: 2, want: StorageRangeResult{storage[1:3], false}},
	}
	for _, test := range tests {
		result := storageRangeAt(state.GetStateObject(addr), test.start, test.end, test.limit)
		if !reflect.DeepEqual(result, &test.want) {
			t.Fatalf("wrong result for range 0x%x..0x%x, limit %d:\ngot %s\nwant %s",
				test.start, test.end, test.limit, dumper.Sdump(result), dumper.Sdump(&test.want))
		}
	}
}

func TestStorageMap(t *testing.T) {
	inserts := storageMap{
		{Key: common.Hash{0}, HashedKey: common.Hash{0, 1}},
		{Key: common.Hash{0}, HashedKey: common.Hash{0, 2}},
		{Key: common.Hash{1}},
		{Key: common.Hash{2}},
		{Key: common.Hash{3}},
	}
	for i := 0; i < 100; i++ {
		var sm storageMap
		for i := range rand.Perm(len(inserts)) {
			sm.insert(inserts[i], 3)
		}
		if !reflect.DeepEqual(sm, inserts[:3]) {
			t.Fatalf("wrong order:\ngot: %s\nwant: %s", dumper.Sdump(sm), dumper.Sdump(inserts[:3]))
		}
	}
}
