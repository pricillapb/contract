// Copyright 2016 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package eth

import (
	"crypto/ecdsa"
	"io/ioutil"
	"math/big"
	"os"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
)

func TestSendTransactionAutoNonce(t *testing.T) {
	var (
		key1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
		key2, _ = crypto.HexToECDSA("8a1f9a8f95be41cd7ccb6168179afb4504aefe388d1e14474d32c45c72ce7b7a")
		// key3, _ = crypto.HexToECDSA("49a7b37aa6f6645917e7b807e9d1c00d4fa71f18343b0d4122a4d2df64dd6fee")
		addr1 = crypto.PubkeyToAddress(key1.PublicKey)
		addr2 = crypto.PubkeyToAddress(key2.PublicKey)
		// addr3   = crypto.PubkeyToAddress(key3.PublicKey)
		db, _ = ethdb.NewMemDatabase()
	)

	// Ensure that key1 has some funds in the genesis block.
	core.WriteGenesisBlockForTesting(db, core.GenesisAccount{addr1, big.NewInt(1000000)})
	evmux := &event.TypeMux{}
	blockchain, _ := core.NewBlockChain(db, core.FakePow{}, evmux)
	gasLimitFn := func() *big.Int {
		// fake block gas limit
		return big.NewInt(50000)
	}
	txPool := core.NewTxPool(evmux, blockchain.State, gasLimitFn)

	// TODO: fix this, it's a bit bad that all this effort is required
	// just to get a tx signed in tests.
	tmpdir, am := tmpAccountManager(key1)
	defer os.RemoveAll(tmpdir)
	am.Unlock(addr1, "")

	api := &PublicTransactionPoolAPI{
		eventMux: evmux,
		chainDb:  db,
		bc:       blockchain,
		miner:    nil,
		am:       am,
		txPool:   txPool,
	}

	tx, err := api.SendTransaction(SendTxArgs{
		From:     addr1,
		To:       addr2,
		Gas:      rpc.NewHexNumber(params.TxGas),
		GasPrice: rpc.NewHexNumber(0),
		Value:    rpc.NewHexNumber(200),
	})
	_, _ = tx, err

	api.SendTransaction(SendTxArgs{
		From:     addr1,
		To:       addr2,
		Gas:      rpc.NewHexNumber(params.TxGas),
		GasPrice: rpc.NewHexNumber(0),
		Value:    rpc.NewHexNumber(200),
	})

	api.SendTransaction(SendTxArgs{
		From:     addr1,
		To:       addr2,
		Gas:      rpc.NewHexNumber(params.TxGas),
		GasPrice: rpc.NewHexNumber(0),
		Value:    rpc.NewHexNumber(200),
	})

	spew.Dump(txPool)
}

// creates account manager holding the given key,
// for testing.
func tmpAccountManager(key *ecdsa.PrivateKey) (tmpdir string, m *accounts.Manager) {
	d, err := ioutil.TempDir("", "eth-keystore-test")
	if err != nil {
		panic(err)
	}
	store := crypto.NewKeyStorePlain(d)
	akey := crypto.NewKeyFromECDSA(key)
	store.StoreKey(akey, "")
	manager := accounts.NewManager(store)
	return d, manager
}
