// Copyright 2014 The go-ethereum Authors
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

package ethdb

import (
	"github.com/dgraph-io/badger"
	"github.com/dgraph-io/badger/options"
	"github.com/golang/snappy"
)

type BadgerDB struct {
	kv *badger.KV
}

type badgerBatch struct {
	kv   *badger.KV
	e    []*badger.Entry
	size int
}

func NewBadger(directory string) (*BadgerDB, error) {
	opts := badger.DefaultOptions
	opts.Dir = directory
	opts.ValueDir = directory
	opts.TableLoadingMode = options.MemoryMap
	opts.SyncWrites = true
	kv, err := badger.NewKV(&opts)
	if err != nil {
		return nil, err
	}
	return &BadgerDB{kv}, nil
}

func (db *BadgerDB) Put(key, value []byte) error {
	return db.kv.Set(key, snappy.Encode(nil, value), 0)
}

func (db *BadgerDB) Get(key []byte) ([]byte, error) {
	var (
		item badger.KVItem
		val  []byte
	)
	err := db.kv.Get(key, &item)
	if err == nil {
		item.Value(func(v []byte) { val, err = snappy.Decode(nil, v) })
	}
	return val, err
}

func (db *BadgerDB) Delete(key []byte) error {
	return db.kv.Delete(key)
}

func (db *BadgerDB) Has(key []byte) (bool, error) {
	return db.kv.Exists(key)
}

func (db *BadgerDB) Close() {
	db.kv.Close()
}

func (db *BadgerDB) NewBatch() Batch {
	return &badgerBatch{db.kv, nil, 0}
}

func (b *badgerBatch) Put(key, val []byte) error {
	b.size += len(val)
	b.e = badger.EntriesSet(b.e, key, snappy.Encode(nil, val))
	return nil
}

func (b *badgerBatch) Write() error {
	e := b.e
	b.e = nil
	if err := b.kv.BatchSet(e); err != nil {
		return err
	}
	for _, entry := range e {
		if entry.Error != nil {
			return entry.Error
		}
	}
	return nil
}

func (b *badgerBatch) ValueSize() int {
	return b.size
}
