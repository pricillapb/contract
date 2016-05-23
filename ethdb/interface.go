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

// Reader wraps database read operations.
type Reader interface {
	Get(key []byte) ([]byte, error)
}

// Writer wraps database write operations.
type Writer interface {
	Put(key, value []byte) error
	Delete(key []byte) error
}

type Database interface {
	Reader // Reading from a database can happen without a transaction.
	Writer // Writing, too.

	Close() // Databases must be closed after use.

	// Writes to the database must be wrapped in a transaction.
	// If a transaction is already in progress, NewTx blocks until
	// the transaction is committed.
	NewTx() (Tx, error)
}

type Tx interface {
	Reader
	Writer // Note that tx writes don't return errors.

	// Commit writes all changes to the underlying database.
	Commit() error
	Discard()
}

func RunTx(db Database, f func(Tx)) error {
	tx, err := db.NewTx()
	if err != nil {
		return err
	}
	f(tx)
	return tx.Commit()
}
