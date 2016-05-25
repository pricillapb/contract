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

package debug

import (
	"runtime/pprof"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/ethdb"
)

var (
	dbReadProfile    = pprof.NewProfile("dbread")
	dbWriteProfile   = pprof.NewProfile("dbwrite")
	dbProfileCounter int64
)

func dbProfileKey() int64 {
	return atomic.AddInt64(&dbProfileCounter, 1)
}

func writeDBProfiles() {
	writeProfile(dbWriteProfile.Name(), "geth.dbwrite.prof")
	writeProfile(dbReadProfile.Name(), "geth.dbread.prof")
}

type profDB struct{ wrapped ethdb.Database }

type profBatch struct{ wrapped ethdb.Batch }

// ProfileDB wraps operations on the given database with
// profiling counters for read and write operations.
func ProfileDB(db ethdb.Database) ethdb.Database {
	return &profDB{wrapped: db}
}

func (db *profDB) Get(key []byte) ([]byte, error) {
	dbReadProfile.Add(dbProfileKey(), 1)
	return db.wrapped.Get(key)
}

func (db *profDB) Put(key, value []byte) error {
	dbWriteProfile.Add(dbProfileKey(), 1)
	return db.wrapped.Put(key, value)
}

func (db *profDB) Delete(key []byte) error {
	dbWriteProfile.Add(dbProfileKey(), 1)
	return db.wrapped.Delete(key)
}

func (db *profDB) Close() {
	db.wrapped.Close()
}

func (db *profDB) NewBatch() ethdb.Batch {
	return &profBatch{wrapped: db.wrapped.NewBatch()}
}

func (batch *profBatch) Put(key, value []byte) error {
	dbWriteProfile.Add(dbProfileKey(), 1)
	return batch.wrapped.Put(key, value)
}

func (batch *profBatch) Write() error {
	return batch.wrapped.Write()
}
