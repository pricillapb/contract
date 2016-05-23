// Copyright 2015 The go-ethereum Authors
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

package core

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/rlp"
)

var (
	headHeaderKey       = []byte("LastHeader")
	headBlockKey        = []byte("LastBlock")
	headFastKey         = []byte("LastFast")
	blockPrefix         = []byte("block-")
	blockNumPrefix      = []byte("block-num-")
	headerSuffix        = []byte("-header")
	bodySuffix          = []byte("-body")
	tdSuffix            = []byte("-td")
	txMetaSuffix        = []byte{0x01}
	receiptsPrefix      = []byte("receipts-")
	blockReceiptsPrefix = []byte("receipts-block-")
	blockHashPrefix     = []byte("block-hash-")      // [deprecated by the header/block split, remove eventually]
	configPrefix        = []byte("ethereum-config-") // config prefix for the db
	mipmapPre           = []byte("mipmap-log-bloom-")

	MIPMapLevels = []uint64{1000000, 500000, 100000, 50000, 1000}
)

type dbWriter interface {
	Put(key, value []byte) error
	Delete(key []byte) error
}

type dbReader interface {
	Get([]byte) ([]byte, error)
}

type dbReadWriter interface {
	dbReader
	dbWriter
}

// GetCanonicalHash retrieves a hash assigned to a canonical block number.
func GetCanonicalHash(db dbReader, number uint64) common.Hash {
	data, _ := db.Get(append(blockNumPrefix, big.NewInt(int64(number)).Bytes()...))
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// GetHeadHeaderHash retrieves the hash of the current canonical head block's
// header. The difference between this and GetHeadBlockHash is that whereas the
// last block hash is only updated upon a full block import, the last header
// hash is updated already at header import, allowing head tracking for the
// light synchronization mechanism.
func GetHeadHeaderHash(db dbReader) common.Hash {
	data, _ := db.Get(headHeaderKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// GetHeadBlockHash retrieves the hash of the current canonical head block.
func GetHeadBlockHash(db dbReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// GetHeadFastBlockHash retrieves the hash of the current canonical head block during
// fast synchronization. The difference between this and GetHeadBlockHash is that
// whereas the last block hash is only updated upon a full block import, the last
// fast hash is updated when importing pre-processed blocks.
func GetHeadFastBlockHash(db dbReader) common.Hash {
	data, _ := db.Get(headFastKey)
	if len(data) == 0 {
		return common.Hash{}
	}
	return common.BytesToHash(data)
}

// GetHeaderRLP retrieves a block header in its raw RLP database encoding, or nil
// if the header's not found.
func GetHeaderRLP(db dbReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(append(append(blockPrefix, hash[:]...), headerSuffix...))
	return data
}

// GetHeader retrieves the block header corresponding to the hash, nil if none
// found.
func GetHeader(db dbReader, hash common.Hash) *types.Header {
	data := GetHeaderRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	header := new(types.Header)
	if err := rlp.Decode(bytes.NewReader(data), header); err != nil {
		glog.V(logger.Error).Infof("invalid block header RLP for hash %x: %v", hash, err)
		return nil
	}
	return header
}

// GetBodyRLP retrieves the block body (transactions and uncles) in RLP encoding.
func GetBodyRLP(db dbReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(append(append(blockPrefix, hash[:]...), bodySuffix...))
	return data
}

// GetBody retrieves the block body (transactons, uncles) corresponding to the
// hash, nil if none found.
func GetBody(db dbReader, hash common.Hash) *types.Body {
	data := GetBodyRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	body := new(types.Body)
	if err := rlp.Decode(bytes.NewReader(data), body); err != nil {
		glog.V(logger.Error).Infof("invalid block body RLP for hash %x: %v", hash, err)
		return nil
	}
	return body
}

// GetTd retrieves a block's total difficulty corresponding to the hash, nil if
// none found.
func GetTd(db dbReader, hash common.Hash) *big.Int {
	data, _ := db.Get(append(append(blockPrefix, hash.Bytes()...), tdSuffix...))
	if len(data) == 0 {
		return nil
	}
	td := new(big.Int)
	if err := rlp.Decode(bytes.NewReader(data), td); err != nil {
		glog.V(logger.Error).Infof("invalid block total difficulty RLP for hash %x: %v", hash, err)
		return nil
	}
	return td
}

// GetBlock retrieves an entire block corresponding to the hash, assembling it
// back from the stored header and body.
func GetBlock(db dbReader, hash common.Hash) *types.Block {
	// Retrieve the block header and body contents
	header := GetHeader(db, hash)
	if header == nil {
		return nil
	}
	body := GetBody(db, hash)
	if body == nil {
		return nil
	}
	// Reassemble the block and return
	return types.NewBlockWithHeader(header).WithBody(body.Transactions, body.Uncles)
}

// GetBlockReceipts retrieves the receipts generated by the transactions included
// in a block given by its hash.
func GetBlockReceipts(db dbReader, hash common.Hash) types.Receipts {
	data, _ := db.Get(append(blockReceiptsPrefix, hash[:]...))
	if len(data) == 0 {
		return nil
	}
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		glog.V(logger.Error).Infof("invalid receipt array RLP for hash %x: %v", hash, err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

// GetTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func GetTransaction(db dbReader, hash common.Hash) (*types.Transaction, common.Hash, uint64, uint64) {
	// Retrieve the transaction itself from the database
	data, _ := db.Get(hash.Bytes())
	if len(data) == 0 {
		return nil, common.Hash{}, 0, 0
	}
	var tx types.Transaction
	if err := rlp.DecodeBytes(data, &tx); err != nil {
		return nil, common.Hash{}, 0, 0
	}
	// Retrieve the blockchain positional metadata
	data, _ = db.Get(append(hash.Bytes(), txMetaSuffix...))
	if len(data) == 0 {
		return nil, common.Hash{}, 0, 0
	}
	var meta struct {
		BlockHash  common.Hash
		BlockIndex uint64
		Index      uint64
	}
	if err := rlp.DecodeBytes(data, &meta); err != nil {
		return nil, common.Hash{}, 0, 0
	}
	return &tx, meta.BlockHash, meta.BlockIndex, meta.Index
}

// GetReceipt returns a receipt by hash
func GetReceipt(db dbReader, txHash common.Hash) *types.Receipt {
	data, _ := db.Get(append(receiptsPrefix, txHash[:]...))
	if len(data) == 0 {
		return nil
	}
	var receipt types.ReceiptForStorage
	err := rlp.DecodeBytes(data, &receipt)
	if err != nil {
		glog.V(logger.Core).Infoln("GetReceipt err:", err)
	}
	return (*types.Receipt)(&receipt)
}

// WriteCanonicalHash stores the canonical hash for the given block number.
func WriteCanonicalHash(db dbWriter, hash common.Hash, number uint64) {
	key := append(blockNumPrefix, big.NewInt(int64(number)).Bytes()...)
	db.Put(key, hash[:])
}

// WriteHeadHeaderHash stores the head header's hash.
func WriteHeadHeaderHash(db dbWriter, hash common.Hash) {
	db.Put(headHeaderKey, hash[:])
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db dbWriter, hash common.Hash) {
	db.Put(headBlockKey, hash[:])
}

// WriteHeadFastBlockHash stores the fast head block's hash.
func WriteHeadFastBlockHash(db dbWriter, hash common.Hash) {
	db.Put(headFastKey, hash[:])
}

// WriteHeader serializes a block header into the database.
func WriteHeader(db dbWriter, header *types.Header) {
	data, err := rlp.EncodeToBytes(header)
	if err != nil {
		panic(err)
	}
	hash := header.Hash()
	key := append(append(blockPrefix, hash[:]...), headerSuffix...)
	db.Put(key, data)
	glog.V(logger.Debug).Infof("stored header #%v [%x…]", header.Number, hash[:4])
}

// WriteBody serializes the body of a block into the database.
func WriteBody(db dbWriter, hash common.Hash, body *types.Body) {
	data, err := rlp.EncodeToBytes(body)
	if err != nil {
		panic(err)
	}
	key := append(append(blockPrefix, hash.Bytes()...), bodySuffix...)
	db.Put(key, data)
	glog.V(logger.Debug).Infof("stored block body [%x…]", hash[:4])
}

// WriteTd serializes the total difficulty of a block into the database.
func WriteTd(db dbWriter, hash common.Hash, td *big.Int) {
	data, err := rlp.EncodeToBytes(td)
	if err != nil {
		panic(err) // td == nil || td < 0
	}
	key := append(append(blockPrefix, hash.Bytes()...), tdSuffix...)
	db.Put(key, data)
	glog.V(logger.Debug).Infof("stored block total difficulty [%x…]: %v", hash[:4], td)
}

// WriteBlock serializes a block into the database, header and body separately.
func WriteBlock(db dbWriter, block *types.Block) {
	// Body is written first to retain database consistency.
	WriteBody(db, block.Hash(), block.Body())
	WriteHeader(db, block.Header())
}

// WriteBlockReceipts stores all the transaction receipts belonging to a block
// as a single receipt slice. This is used during chain reorganisations for
// rescheduling dropped transactions.
func WriteBlockReceipts(db dbWriter, hash common.Hash, receipts []*types.Receipt) {
	sreceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i := range receipts {
		sreceipts[i] = (*types.ReceiptForStorage)(receipts[i])
	}
	bytes, err := rlp.EncodeToBytes(sreceipts)
	if err != nil {
		panic(err)
	}
	db.Put(append(blockReceiptsPrefix, hash.Bytes()...), bytes)
	glog.V(logger.Debug).Infof("stored block receipts [%x…]", hash[:4])
}

// WriteTransactions stores the transactions associated with a specific block
// into the given database. Beside writing the transaction, the function also
// stores a metadata entry along with the transaction, detailing the position
// of this within the blockchain.
func WriteTransactions(db dbWriter, block *types.Block) {
	type txMeta struct {
		BlockHash  common.Hash
		BlockIndex uint64
		Index      uint64
	}

	// Iterate over each transaction and encode it with its metadata
	var buf bytes.Buffer
	for i, tx := range block.Transactions() {
		buf.Reset()
		if err := rlp.Encode(&buf, tx); err != nil {
			panic(err)
		}
		db.Put(tx.Hash().Bytes(), buf.Bytes())

		buf.Reset()
		meta := txMeta{block.Hash(), block.NumberU64(), uint64(i)}
		if err := rlp.Encode(&buf, meta); err != nil {
			panic(err)
		}
		db.Put(append(tx.Hash().Bytes(), txMetaSuffix...), buf.Bytes())
	}
}

// WriteReceipts stores a batch of transaction receipts into the database.
func WriteReceipts(db dbWriter, receipts types.Receipts) {
	var buf bytes.Buffer
	for _, receipt := range receipts {
		buf.Reset()
		if err := rlp.Encode(&buf, (*types.ReceiptForStorage)(receipt)); err != nil {
			panic(err)
		}
		db.Put(append(receiptsPrefix, receipt.TxHash.Bytes()...), buf.Bytes())
	}
}

// DeleteCanonicalHash removes the number to hash canonical mapping.
func DeleteCanonicalHash(db dbWriter, number uint64) {
	db.Delete(append(blockNumPrefix, big.NewInt(int64(number)).Bytes()...))
}

// DeleteHeader removes all block header data associated with a hash.
func DeleteHeader(db dbWriter, hash common.Hash) {
	db.Delete(append(append(blockPrefix, hash[:]...), headerSuffix...))
}

// DeleteBody removes all block body data associated with a hash.
func DeleteBody(db dbWriter, hash common.Hash) {
	db.Delete(append(append(blockPrefix, hash.Bytes()...), bodySuffix...))
}

// DeleteTd removes all block total difficulty data associated with a hash.
func DeleteTd(db dbWriter, hash common.Hash) {
	db.Delete(append(append(blockPrefix, hash.Bytes()...), tdSuffix...))
}

// DeleteBlock removes all block data associated with a hash.
func DeleteBlock(db dbWriter, hash common.Hash) {
	DeleteBlockReceipts(db, hash)
	DeleteHeader(db, hash)
	DeleteBody(db, hash)
	DeleteTd(db, hash)
}

// DeleteBlockReceipts removes all receipt data associated with a block hash.
func DeleteBlockReceipts(db dbWriter, hash common.Hash) {
	db.Delete(append(blockReceiptsPrefix, hash.Bytes()...))
}

// DeleteTransaction removes all transaction data associated with a hash.
func DeleteTransaction(db dbWriter, hash common.Hash) {
	db.Delete(hash.Bytes())
	db.Delete(append(hash.Bytes(), txMetaSuffix...))
}

// DeleteReceipt removes all receipt data associated with a transaction hash.
func DeleteReceipt(db dbWriter, hash common.Hash) {
	db.Delete(append(receiptsPrefix, hash.Bytes()...))
}

// [deprecated by the header/block split, remove eventually]
// GetBlockByHashOld returns the old combined block corresponding to the hash
// or nil if not found. This method is only used by the upgrade mechanism to
// access the old combined block representation. It will be dropped after the
// network transitions to eth/63.
func GetBlockByHashOld(db dbReader, hash common.Hash) *types.Block {
	data, _ := db.Get(append(blockHashPrefix, hash[:]...))
	if len(data) == 0 {
		return nil
	}
	var block types.StorageBlock
	if err := rlp.Decode(bytes.NewReader(data), &block); err != nil {
		glog.V(logger.Error).Infof("invalid block RLP for hash %x: %v", hash, err)
		return nil
	}
	return (*types.Block)(&block)
}

// returns a formatted MIP mapped key by adding prefix, canonical number and level
//
// ex. fn(98, 1000) = (prefix || 1000 || 0)
func mipmapKey(num, level uint64) []byte {
	lkey := make([]byte, 8)
	binary.BigEndian.PutUint64(lkey, level)
	key := new(big.Int).SetUint64(num / level * level)

	return append(mipmapPre, append(lkey, key.Bytes()...)...)
}

// WriteMapmapBloom writes each address included in the receipts' logs to the
// MIP bloom bin.
func WriteMipmapBloom(db dbReadWriter, number uint64, receipts types.Receipts) {
	for _, level := range MIPMapLevels {
		key := mipmapKey(number, level)
		bloomDat, _ := db.Get(key)
		bloom := types.BytesToBloom(bloomDat)
		for _, receipt := range receipts {
			for _, log := range receipt.Logs {
				bloom.Add(log.Address.Big())
			}
		}
		db.Put(key, bloom.Bytes())
	}
}

// GetMipmapBloom returns a bloom filter using the number and level as input
// parameters. For available levels see MIPMapLevels.
func GetMipmapBloom(db dbReader, number, level uint64) types.Bloom {
	bloomDat, _ := db.Get(mipmapKey(number, level))
	return types.BytesToBloom(bloomDat)
}

// GetBlockChainVersion reads the version number from db.
func GetBlockChainVersion(db dbReader) int {
	var vsn uint
	enc, _ := db.Get([]byte("BlockchainVersion"))
	rlp.DecodeBytes(enc, &vsn)
	return int(vsn)
}

// WriteBlockChainVersion writes vsn as the version number to db.
func WriteBlockChainVersion(db dbWriter, vsn int) {
	enc, _ := rlp.EncodeToBytes(uint(vsn))
	db.Put([]byte("BlockchainVersion"), enc)
}

// WriteChainConfig writes the chain config settings to the database.
func WriteChainConfig(db dbWriter, hash common.Hash, cfg *ChainConfig) {
	// short circuit and ignore if nil config. GetChainConfig
	// will return a default.
	if cfg == nil {
		return
	}

	jsonChainConfig, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	db.Put(append(configPrefix, hash[:]...), jsonChainConfig)
}

// GetChainConfig will fetch the network settings based on the given hash.
func GetChainConfig(db dbReader, hash common.Hash) (*ChainConfig, error) {
	jsonChainConfig, _ := db.Get(append(configPrefix, hash[:]...))
	if len(jsonChainConfig) == 0 {
		return nil, ChainConfigNotFoundErr
	}

	var config ChainConfig
	if err := json.Unmarshal(jsonChainConfig, &config); err != nil {
		return nil, err
	}
	return &config, nil
}
