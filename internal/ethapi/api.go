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

package ethapi

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/ethash"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/hexutil"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/syndtr/goleveldb/leveldb"
	"golang.org/x/net/context"
)

var defaultGas = (*hexutil.Big)(big.NewInt(90000))

// PublicEthereumAPI provides an API to access Ethereum related information.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicEthereumAPI struct {
	b Backend
}

// NewPublicEthereumAPI creates a new Etheruem protocol API.
func NewPublicEthereumAPI(b Backend) *PublicEthereumAPI {
	return &PublicEthereumAPI{b}
}

// GasPrice returns a suggestion for a gas price.
func (s *PublicEthereumAPI) GasPrice(ctx context.Context) (*hexutil.Big, error) {
	price, err := s.b.SuggestPrice(ctx)
	return (*hexutil.Big)(price), err
}

// ProtocolVersion returns the current Ethereum protocol version this node supports
func (s *PublicEthereumAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(s.b.ProtocolVersion())
}

// Syncing returns false in case the node is currently not syncing with the network. It can be up to date or has not
// yet received the latest block headers from its pears. In case it is synchronizing:
// - startingBlock: block number this node started to synchronise from
// - currentBlock:  block number this node is currently importing
// - highestBlock:  block number of the highest block header this node has received from peers
// - pulledStates:  number of state entries processed until now
// - knownStates:   number of known state entries that still need to be pulled
func (s *PublicEthereumAPI) Syncing() (interface{}, error) {
	progress := s.b.Downloader().Progress()

	// Return not syncing if the synchronisation already completed
	if progress.CurrentBlock >= progress.HighestBlock {
		return false, nil
	}
	// Otherwise gather the block sync stats
	return map[string]hexutil.Uint{
		"startingBlock": hexutil.Uint(progress.StartingBlock),
		"currentBlock":  hexutil.Uint(progress.CurrentBlock),
		"highestBlock":  hexutil.Uint(progress.HighestBlock),
		"pulledStates":  hexutil.Uint(progress.PulledStates),
		"knownStates":   hexutil.Uint(progress.KnownStates),
	}, nil
}

// PublicTxPoolAPI offers and API for the transaction pool. It only operates on data that is non confidential.
type PublicTxPoolAPI struct {
	b Backend
}

// NewPublicTxPoolAPI creates a new tx pool service that gives information about the transaction pool.
func NewPublicTxPoolAPI(b Backend) *PublicTxPoolAPI {
	return &PublicTxPoolAPI{b}
}

// Content returns the transactions contained within the transaction pool.
func (s *PublicTxPoolAPI) Content() map[string]map[string]map[string]*RPCTransaction {
	content := map[string]map[string]map[string]*RPCTransaction{
		"pending": make(map[string]map[string]*RPCTransaction),
		"queued":  make(map[string]map[string]*RPCTransaction),
	}
	pending, queue := s.b.TxPoolContent()

	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]*RPCTransaction)
		for nonce, tx := range txs {
			dump[fmt.Sprintf("%d", nonce)] = newRPCPendingTransaction(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]*RPCTransaction)
		for nonce, tx := range txs {
			dump[fmt.Sprintf("%d", nonce)] = newRPCPendingTransaction(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// Status returns the number of pending and queued transaction in the pool.
func (s *PublicTxPoolAPI) Status() map[string]hexutil.Uint {
	pending, queue := s.b.Stats()
	return map[string]hexutil.Uint{
		"pending": hexutil.Uint(pending),
		"queued":  hexutil.Uint(queue),
	}
}

// Inspect retrieves the content of the transaction pool and flattens it into an
// easily inspectable list.
func (s *PublicTxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"pending": make(map[string]map[string]string),
		"queued":  make(map[string]map[string]string),
	}
	pending, queue := s.b.TxPoolContent()

	// Define a formatter to flatten a transaction into a string
	var format = func(tx *types.Transaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v wei + %v × %v gas", tx.To().Hex(), tx.Value(), tx.Gas(), tx.GasPrice())
		}
		return fmt.Sprintf("contract creation: %v wei + %v × %v gas", tx.Value(), tx.Gas(), tx.GasPrice())
	}
	// Flatten the pending transactions
	for account, txs := range pending {
		dump := make(map[string]string)
		for nonce, tx := range txs {
			dump[fmt.Sprintf("%d", nonce)] = format(tx)
		}
		content["pending"][account.Hex()] = dump
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]string)
		for nonce, tx := range txs {
			dump[fmt.Sprintf("%d", nonce)] = format(tx)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// PublicAccountAPI provides an API to access accounts managed by this node.
// It offers only methods that can retrieve accounts.
type PublicAccountAPI struct {
	am *accounts.Manager
}

// NewPublicAccountAPI creates a new PublicAccountAPI.
func NewPublicAccountAPI(am *accounts.Manager) *PublicAccountAPI {
	return &PublicAccountAPI{am: am}
}

// Accounts returns the collection of accounts this node manages
func (s *PublicAccountAPI) Accounts() []accounts.Account {
	return s.am.Accounts()
}

// PrivateAccountAPI provides an API to access accounts managed by this node.
// It offers methods to create, (un)lock en list accounts. Some methods accept
// passwords and are therefore considered private by default.
type PrivateAccountAPI struct {
	am *accounts.Manager
	b  Backend
}

// NewPrivateAccountAPI create a new PrivateAccountAPI.
func NewPrivateAccountAPI(b Backend) *PrivateAccountAPI {
	return &PrivateAccountAPI{
		am: b.AccountManager(),
		b:  b,
	}
}

// ListAccounts will return a list of addresses for accounts this node manages.
func (s *PrivateAccountAPI) ListAccounts() []common.Address {
	accounts := s.am.Accounts()
	addresses := make([]common.Address, len(accounts))
	for i, acc := range accounts {
		addresses[i] = acc.Address
	}
	return addresses
}

// NewAccount will create a new account and returns the address for the new account.
func (s *PrivateAccountAPI) NewAccount(password string) (common.Address, error) {
	acc, err := s.am.NewAccount(password)
	if err == nil {
		return acc.Address, nil
	}
	return common.Address{}, err
}

// ImportRawKey stores the given hex encoded ECDSA key into the key directory,
// encrypting it with the passphrase.
func (s *PrivateAccountAPI) ImportRawKey(privkey string, password string) (common.Address, error) {
	hexkey, err := hex.DecodeString(privkey)
	if err != nil {
		return common.Address{}, err
	}

	acc, err := s.am.ImportECDSA(crypto.ToECDSA(hexkey), password)
	return acc.Address, err
}

// UnlockAccount will unlock the account associated with the given address with
// the given password for duration seconds. If duration is nil it will use a
// default of 300 seconds. It returns an indication if the account was unlocked.
func (s *PrivateAccountAPI) UnlockAccount(addr common.Address, password string, duration *hexutil.Uint) (bool, error) {
	seconds := 300
	if duration != nil {
		if *duration > math.MaxInt32 {
			return false, errors.New("insane unlock duration, use 0x0 to unlock forever")
		}
		seconds = int(*duration)
	}
	a := accounts.Account{Address: addr}
	d := time.Duration(seconds) * time.Second
	if err := s.am.TimedUnlock(a, password, d); err != nil {
		return false, err
	}
	return true, nil
}

// LockAccount will lock the account associated with the given address when it's unlocked.
func (s *PrivateAccountAPI) LockAccount(addr common.Address) bool {
	return s.am.Lock(addr) == nil
}

// SendTransaction will create a transaction from the given arguments and
// tries to sign it with the key associated with args.To. If the given passwd isn't
// able to decrypt the key it fails.
func (s *PrivateAccountAPI) SendTransaction(ctx context.Context, args SendTxArgs, passwd string) (common.Hash, error) {
	var err error
	args, err = prepareSendTxArgs(ctx, args, s.b)
	if err != nil {
		return common.Hash{}, err
	}

	if args.Nonce == nil {
		nonce, err := s.b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return common.Hash{}, err
		}
		args.Nonce = hexUintPointer(nonce)
	}

	var tx *types.Transaction
	if args.To == nil {
		tx = types.NewContractCreation(uint64(*args.Nonce), args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	} else {
		tx = types.NewTransaction(uint64(*args.Nonce), *args.To, args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	}

	signature, err := s.am.SignWithPassphrase(args.From, passwd, tx.SigHash().Bytes())
	if err != nil {
		return common.Hash{}, err
	}

	return submitTransaction(ctx, s.b, tx, signature)
}

// SignAndSendTransaction was renamed to SendTransaction. This method is deprecated
// and will be removed in the future. It primary goal is to give clients time to update.
func (s *PrivateAccountAPI) SignAndSendTransaction(ctx context.Context, args SendTxArgs, passwd string) (common.Hash, error) {
	return s.SendTransaction(ctx, args, passwd)
}

// PublicBlockChainAPI provides an API to access the Ethereum blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type PublicBlockChainAPI struct {
	b Backend
}

// NewPublicBlockChainAPI creates a new Etheruem blockchain API.
func NewPublicBlockChainAPI(b Backend) *PublicBlockChainAPI {
	return &PublicBlockChainAPI{b}
}

// BlockNumber returns the block number of the chain head.
func (s *PublicBlockChainAPI) BlockNumber() *big.Int {
	return s.b.HeaderByNumber(rpc.LatestBlockNumber).Number
}

// GetBalance returns the amount of wei for the given address in the state of the
// given block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta
// block numbers are also allowed.
func (s *PublicBlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*big.Int, error) {
	state, _, err := s.b.StateAndHeaderByNumber(blockNr)
	if state == nil || err != nil {
		return nil, err
	}

	return state.GetBalance(ctx, address)
}

// GetBlockByNumber returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByNumber(ctx context.Context, blockNr rpc.BlockNumber, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		response, err := s.rpcOutputBlock(block, true, fullTx)
		if err == nil && blockNr == rpc.PendingBlockNumber {
			// Pending blocks need to nil out a few fields
			for _, field := range []string{"hash", "nonce", "logsBloom", "miner"} {
				response[field] = nil
			}
		}
		return response, err
	}
	return nil, err
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetBlockByHash(ctx context.Context, blockHash common.Hash, fullTx bool) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		return s.rpcOutputBlock(block, true, fullTx)
	}
	return nil, err
}

// GetUncleByBlockNumberAndIndex returns the uncle block for the given block hash and index. When fullTx is true
// all transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetUncleByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.BlockByNumber(ctx, blockNr)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			glog.V(logger.Debug).Infof("uncle block on index %d not found for block #%d", index, blockNr)
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}

// GetUncleByBlockHashAndIndex returns the uncle block for the given block hash and index. When fullTx is true
// all transactions in the block are returned in full detail, otherwise only the transaction hash is returned.
func (s *PublicBlockChainAPI) GetUncleByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (map[string]interface{}, error) {
	block, err := s.b.GetBlock(ctx, blockHash)
	if block != nil {
		uncles := block.Uncles()
		if index >= hexutil.Uint(len(uncles)) {
			glog.V(logger.Debug).Infof("uncle block on index %d not found for block %s", index, blockHash.Hex())
			return nil, nil
		}
		block = types.NewBlockWithHeader(uncles[index])
		return s.rpcOutputBlock(block, false, false)
	}
	return nil, err
}

// GetUncleCountByBlockNumber returns number of uncles in the block for the given block number
func (s *PublicBlockChainAPI) GetUncleCountByBlockNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return hexUintPointer(uint64(len(block.Uncles())))
	}
	return nil
}

// GetUncleCountByBlockHash returns number of uncles in the block for the given block hash
func (s *PublicBlockChainAPI) GetUncleCountByBlockHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return hexUintPointer(uint64(len(block.Uncles())))
	}
	return nil
}

// GetCode returns the code stored at the given address in the state for the given block number.
func (s *PublicBlockChainAPI) GetCode(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	state, _, err := s.b.StateAndHeaderByNumber(blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	return state.GetCode(ctx, address)
}

// GetStorageAt returns the storage from the state at the given address, key and
// block number. The rpc.LatestBlockNumber and rpc.PendingBlockNumber meta block
// numbers are also allowed.
func (s *PublicBlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, key common.Hash, blockNr rpc.BlockNumber) (common.Hash, error) {
	state, _, err := s.b.StateAndHeaderByNumber(blockNr)
	if state == nil || err != nil {
		return common.Hash{}, err
	}
	return state.GetState(ctx, address, key)
}

// callmsg is the message type used for call transations.
type callmsg struct {
	addr          common.Address
	to            *common.Address
	gas, gasPrice *big.Int
	value         *big.Int
	data          []byte
}

// accessor boilerplate to implement core.Message
func (m callmsg) From() (common.Address, error)         { return m.addr, nil }
func (m callmsg) FromFrontier() (common.Address, error) { return m.addr, nil }
func (m callmsg) Nonce() uint64                         { return 0 }
func (m callmsg) CheckNonce() bool                      { return false }
func (m callmsg) To() *common.Address                   { return m.to }
func (m callmsg) GasPrice() *big.Int                    { return m.gasPrice }
func (m callmsg) Gas() *big.Int                         { return m.gas }
func (m callmsg) Value() *big.Int                       { return m.value }
func (m callmsg) Data() []byte                          { return m.data }

// CallArgs represents the arguments for a call.
type CallArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      hexutil.Big     `json:"gas"`
	GasPrice hexutil.Big     `json:"gasPrice"`
	Value    hexutil.Big     `json:"value"`
	Data     hexutil.Bytes   `json:"data"`
}

func (s *PublicBlockChainAPI) doCall(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, *big.Int, error) {
	state, header, err := s.b.StateAndHeaderByNumber(blockNr)
	if state == nil || err != nil {
		return nil, common.Big0, err
	}

	// Set the account address to interact with
	var addr common.Address
	if args.From == (common.Address{}) {
		accounts := s.b.AccountManager().Accounts()
		if len(accounts) == 0 {
			addr = common.Address{}
		} else {
			addr = accounts[0].Address
		}
	} else {
		addr = args.From
	}

	// Assemble the CALL invocation
	msg := callmsg{
		addr:     addr,
		to:       args.To,
		gas:      args.Gas.ToInt(),
		gasPrice: args.GasPrice.ToInt(),
		value:    args.Value.ToInt(),
		data:     args.Data,
	}
	if msg.gas.Cmp(common.Big0) == 0 {
		msg.gas = big.NewInt(50000000)
	}
	if msg.gasPrice.Cmp(common.Big0) == 0 {
		msg.gasPrice = new(big.Int).Mul(big.NewInt(50), common.Shannon)
	}

	// Execute the call and return
	vmenv, vmError, err := s.b.GetVMEnv(ctx, msg, state, header)
	if err != nil {
		return nil, common.Big0, err
	}
	gp := new(core.GasPool).AddGas(common.MaxBig)
	res, gas, err := core.ApplyMessage(vmenv, msg, gp)
	if vmerr := vmError(); vmerr != nil {
		return nil, common.Big0, vmerr
	}
	return res, gas, err
}

// Call executes the given transaction on the state for the given block number.
// It doesn't make and changes in the state/blockchain and is usefull to execute and retrieve values.
func (s *PublicBlockChainAPI) Call(ctx context.Context, args CallArgs, blockNr rpc.BlockNumber) (hexutil.Bytes, error) {
	result, _, err := s.doCall(ctx, args, blockNr)
	return result, err
}

// EstimateGas returns an estimate of the amount of gas needed to execute the given transaction.
func (s *PublicBlockChainAPI) EstimateGas(ctx context.Context, args CallArgs) (*hexutil.Big, error) {
	_, gas, err := s.doCall(ctx, args, rpc.PendingBlockNumber)
	return (*hexutil.Big)(gas), err
}

// ExecutionResult groups all structured logs emitted by the EVM
// while replaying a transaction in debug mode as well as the amount of
// gas used and the return value
type ExecutionResult struct {
	Gas         *big.Int       `json:"gas"`
	ReturnValue hexutil.Bytes  `json:"returnValue"`
	StructLogs  []StructLogRes `json:"structLogs"`
}

// StructLogRes stores a structured log emitted by the EVM while replaying a
// transaction in debug mode
type StructLogRes struct {
	Pc      uint64            `json:"pc"`
	Op      string            `json:"op"`
	Gas     *big.Int          `json:"gas"`
	GasCost *big.Int          `json:"gasCost"`
	Depth   int               `json:"depth"`
	Error   error             `json:"error"`
	Stack   []string          `json:"stack"`
	Memory  []string          `json:"memory"`
	Storage map[string]string `json:"storage"`
}

// formatLogs formats EVM returned structured logs for json output
func FormatLogs(structLogs []vm.StructLog) []StructLogRes {
	formattedStructLogs := make([]StructLogRes, len(structLogs))
	for index, trace := range structLogs {
		formattedStructLogs[index] = StructLogRes{
			Pc:      trace.Pc,
			Op:      trace.Op.String(),
			Gas:     trace.Gas,
			GasCost: trace.GasCost,
			Depth:   trace.Depth,
			Error:   trace.Err,
			Stack:   make([]string, len(trace.Stack)),
			Storage: make(map[string]string),
		}

		for i, stackValue := range trace.Stack {
			formattedStructLogs[index].Stack[i] = fmt.Sprintf("%x", common.LeftPadBytes(stackValue.Bytes(), 32))
		}

		for i := 0; i+32 <= len(trace.Memory); i += 32 {
			formattedStructLogs[index].Memory = append(formattedStructLogs[index].Memory, fmt.Sprintf("%x", trace.Memory[i:i+32]))
		}

		for i, storageValue := range trace.Storage {
			formattedStructLogs[index].Storage[fmt.Sprintf("%x", i)] = fmt.Sprintf("%x", storageValue)
		}
	}
	return formattedStructLogs
}

// rpcOutputBlock converts the given block to the RPC output which depends on fullTx. If inclTx is true transactions are
// returned. When fullTx is true the returned block contains full transaction details, otherwise it will only contain
// transaction hashes.
func (s *PublicBlockChainAPI) rpcOutputBlock(b *types.Block, inclTx bool, fullTx bool) (map[string]interface{}, error) {
	head := b.Header() // copies the header once
	fields := map[string]interface{}{
		"number":           (*hexutil.Big)(head.Number),
		"hash":             b.Hash(),
		"parentHash":       head.ParentHash,
		"nonce":            head.Nonce,
		"mixHash":          head.MixDigest,
		"sha3Uncles":       head.UncleHash,
		"logsBloom":        head.Bloom,
		"stateRoot":        head.Root,
		"miner":            head.Coinbase,
		"difficulty":       (*hexutil.Big)(head.Difficulty),
		"totalDifficulty":  (*hexutil.Big)(s.b.GetTd(b.Hash())),
		"extraData":        hexutil.Bytes(head.Extra),
		"size":             hexutil.Uint(b.Size()),
		"gasLimit":         (*hexutil.Big)(head.GasLimit),
		"gasUsed":          (*hexutil.Big)(head.GasUsed),
		"timestamp":        (*hexutil.Big)(head.Time),
		"transactionsRoot": head.TxHash,
		"receiptRoot":      head.ReceiptHash,
	}

	if inclTx {
		formatTx := func(tx *types.Transaction) (interface{}, error) {
			return tx.Hash(), nil
		}

		if fullTx {
			formatTx = func(tx *types.Transaction) (interface{}, error) {
				return newRPCTransaction(b, tx.Hash())
			}
		}

		txs := b.Transactions()
		transactions := make([]interface{}, len(txs))
		var err error
		for i, tx := range b.Transactions() {
			if transactions[i], err = formatTx(tx); err != nil {
				return nil, err
			}
		}
		fields["transactions"] = transactions
	}

	uncles := b.Uncles()
	uncleHashes := make([]common.Hash, len(uncles))
	for i, uncle := range uncles {
		uncleHashes[i] = uncle.Hash()
	}
	fields["uncles"] = uncleHashes

	return fields, nil
}

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	BlockHash        common.Hash     `json:"blockHash"`
	BlockNumber      *hexutil.Big    `json:"blockNumber"`
	From             common.Address  `json:"from"`
	Gas              *hexutil.Big    `json:"gas"`
	GasPrice         *hexutil.Big    `json:"gasPrice"`
	Hash             common.Hash     `json:"hash"`
	Input            hexutil.Bytes   `json:"input"`
	Nonce            hexutil.Uint    `json:"nonce"`
	To               *common.Address `json:"to"`
	TransactionIndex hexutil.Uint    `json:"transactionIndex"`
	Value            *hexutil.Big    `json:"value"`
	V                hexutil.Uint    `json:"v"`
	R                *hexutil.Big    `json:"r"`
	S                *hexutil.Big    `json:"s"`
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *types.Transaction) *RPCTransaction {
	from, _ := tx.FromFrontier()
	v, r, s := tx.SignatureValues()
	return &RPCTransaction{
		From:     from,
		Gas:      (*hexutil.Big)(tx.Gas()),
		GasPrice: (*hexutil.Big)(tx.GasPrice()),
		Hash:     tx.Hash(),
		Input:    hexutil.Bytes(tx.Data()),
		Nonce:    hexutil.Uint(tx.Nonce()),
		To:       tx.To(),
		Value:    (*hexutil.Big)(tx.Value()),
		V:        hexutil.Uint(v),
		R:        (*hexutil.Big)(r),
		S:        (*hexutil.Big)(s),
	}
}

// newRPCTransaction returns a transaction that will serialize to the RPC representation.
func newRPCTransactionFromBlockIndex(b *types.Block, txIndex uint64) (*RPCTransaction, error) {
	if txIndex >= 0 && txIndex < uint64(len(b.Transactions())) {
		tx := b.Transactions()[txIndex]
		from, err := tx.FromFrontier()
		if err != nil {
			return nil, err
		}
		v, r, s := tx.SignatureValues()
		return &RPCTransaction{
			BlockHash:        b.Hash(),
			BlockNumber:      (*hexutil.Big)(b.Number()),
			From:             from,
			Gas:              (*hexutil.Big)(tx.Gas()),
			GasPrice:         (*hexutil.Big)(tx.GasPrice()),
			Hash:             tx.Hash(),
			Input:            hexutil.Bytes(tx.Data()),
			Nonce:            hexutil.Uint(tx.Nonce()),
			To:               tx.To(),
			TransactionIndex: hexutil.Uint(txIndex),
			Value:            (*hexutil.Big)(tx.Value()),
			V:                hexutil.Uint(v),
			R:                (*hexutil.Big)(r),
			S:                (*hexutil.Big)(s),
		}, nil
	}

	return nil, nil
}

// newRPCTransaction returns a transaction that will serialize to the RPC representation.
func newRPCTransaction(b *types.Block, txHash common.Hash) (*RPCTransaction, error) {
	for idx, tx := range b.Transactions() {
		if tx.Hash() == txHash {
			return newRPCTransactionFromBlockIndex(b, uint64(idx))
		}
	}

	return nil, nil
}

// PublicTransactionPoolAPI exposes methods for the RPC interface
type PublicTransactionPoolAPI struct {
	b Backend
}

// NewPublicTransactionPoolAPI creates a new RPC service with methods specific for the transaction pool.
func NewPublicTransactionPoolAPI(b Backend) *PublicTransactionPoolAPI {
	return &PublicTransactionPoolAPI{b}
}

func getTransaction(chainDb ethdb.Database, b Backend, txHash common.Hash) (*types.Transaction, bool, error) {
	txData, err := chainDb.Get(txHash.Bytes())
	isPending := false
	tx := new(types.Transaction)

	if err == nil && len(txData) > 0 {
		if err := rlp.DecodeBytes(txData, tx); err != nil {
			return nil, isPending, err
		}
	} else {
		// pending transaction?
		tx = b.GetPoolTransaction(txHash)
		isPending = true
	}

	return tx, isPending, nil
}

// GetBlockTransactionCountByNumber returns the number of transactions in the block with the given block number.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByNumber(ctx context.Context, blockNr rpc.BlockNumber) *hexutil.Uint {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return hexUintPointer(uint64(len(block.Transactions())))
	}
	return nil
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *PublicTransactionPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return hexUintPointer(uint64(len(block.Transactions())))
	}
	return nil
}

// GetTransactionByBlockNumberAndIndex returns the transaction for the given block number and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockNumberAndIndex(ctx context.Context, blockNr rpc.BlockNumber, index hexutil.Uint) (*RPCTransaction, error) {
	if block, _ := s.b.BlockByNumber(ctx, blockNr); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil, nil
}

// GetTransactionByBlockHashAndIndex returns the transaction for the given block hash and index.
func (s *PublicTransactionPoolAPI) GetTransactionByBlockHashAndIndex(ctx context.Context, blockHash common.Hash, index hexutil.Uint) (*RPCTransaction, error) {
	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransactionFromBlockIndex(block, uint64(index))
	}
	return nil, nil
}

// GetTransactionCount returns the number of transactions the given address has sent for the given block number
func (s *PublicTransactionPoolAPI) GetTransactionCount(ctx context.Context, address common.Address, blockNr rpc.BlockNumber) (*hexutil.Uint, error) {
	state, _, err := s.b.StateAndHeaderByNumber(blockNr)
	if state == nil || err != nil {
		return nil, err
	}
	nonce, err := state.GetNonce(ctx, address)
	if err != nil {
		return nil, err
	}
	return hexUintPointer(nonce), nil
}

// getTransactionBlockData fetches the meta data for the given transaction from the chain database. This is useful to
// retrieve block information for a hash. It returns the block hash, block index and transaction index.
func getTransactionBlockData(chainDb ethdb.Database, txHash common.Hash) (common.Hash, uint64, uint64, error) {
	var txBlock struct {
		BlockHash  common.Hash
		BlockIndex uint64
		Index      uint64
	}

	blockData, err := chainDb.Get(append(txHash.Bytes(), 0x0001))
	if err != nil {
		return common.Hash{}, uint64(0), uint64(0), err
	}

	reader := bytes.NewReader(blockData)
	if err = rlp.Decode(reader, &txBlock); err != nil {
		return common.Hash{}, uint64(0), uint64(0), err
	}

	return txBlock.BlockHash, txBlock.BlockIndex, txBlock.Index, nil
}

// GetTransactionByHash returns the transaction for the given hash
func (s *PublicTransactionPoolAPI) GetTransactionByHash(ctx context.Context, txHash common.Hash) (*RPCTransaction, error) {
	var tx *types.Transaction
	var isPending bool
	var err error

	if tx, isPending, err = getTransaction(s.b.ChainDb(), s.b, txHash); err != nil {
		glog.V(logger.Debug).Infof("%v\n", err)
		return nil, nil
	} else if tx == nil {
		return nil, nil
	}

	if isPending {
		return newRPCPendingTransaction(tx), nil
	}

	blockHash, _, _, err := getTransactionBlockData(s.b.ChainDb(), txHash)
	if err != nil {
		glog.V(logger.Debug).Infof("%v\n", err)
		return nil, nil
	}

	if block, _ := s.b.GetBlock(ctx, blockHash); block != nil {
		return newRPCTransaction(block, txHash)
	}

	return nil, nil
}

// GetTransactionReceipt returns the transaction receipt for the given transaction hash.
func (s *PublicTransactionPoolAPI) GetTransactionReceipt(txHash common.Hash) (map[string]interface{}, error) {
	receipt := core.GetReceipt(s.b.ChainDb(), txHash)
	if receipt == nil {
		glog.V(logger.Debug).Infof("receipt not found for transaction %s", txHash.Hex())
		return nil, nil
	}

	tx, _, err := getTransaction(s.b.ChainDb(), s.b, txHash)
	if err != nil {
		glog.V(logger.Debug).Infof("%v\n", err)
		return nil, nil
	}

	txBlock, blockIndex, index, err := getTransactionBlockData(s.b.ChainDb(), txHash)
	if err != nil {
		glog.V(logger.Debug).Infof("%v\n", err)
		return nil, nil
	}

	from, err := tx.FromFrontier()
	if err != nil {
		glog.V(logger.Debug).Infof("%v\n", err)
		return nil, nil
	}

	fields := map[string]interface{}{
		"root":              hexutil.Bytes(receipt.PostState),
		"blockHash":         txBlock,
		"blockNumber":       hexutil.Uint(blockIndex),
		"transactionHash":   txHash,
		"transactionIndex":  hexutil.Uint(index),
		"from":              from,
		"to":                tx.To(),
		"gasUsed":           (*hexutil.Big)(receipt.GasUsed),
		"cumulativeGasUsed": (*hexutil.Big)(receipt.CumulativeGasUsed),
		"contractAddress":   nil,
		"logs":              receipt.Logs,
		"logsBloom":         receipt.Bloom,
	}
	if receipt.Logs == nil {
		fields["logs"] = []vm.Logs{}
	}
	// If the ContractAddress is 20 0x0 bytes, assume it is not a contract creation
	if receipt.ContractAddress != (common.Address{}) {
		fields["contractAddress"] = receipt.ContractAddress
	}
	return fields, nil
}

// sign is a helper function that signs a transaction with the private key of the given address.
func (s *PublicTransactionPoolAPI) sign(addr common.Address, tx *types.Transaction) (*types.Transaction, error) {
	signature, err := s.b.AccountManager().Sign(addr, tx.SigHash().Bytes())
	if err != nil {
		return nil, err
	}
	return tx.WithSignature(signature)
}

// SendTxArgs represents the arguments to sumbit a new transaction into the transaction pool.
type SendTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Big    `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Data     hexutil.Bytes   `json:"data"`
	Nonce    *hexutil.Uint   `json:"nonce"`
}

// prepareSendTxArgs is a helper function that fills in default values for unspecified tx fields.
func prepareSendTxArgs(ctx context.Context, args SendTxArgs, b Backend) (SendTxArgs, error) {
	if args.Gas == nil {
		args.Gas = defaultGas
	}
	if args.GasPrice == nil {
		price, err := b.SuggestPrice(ctx)
		if err != nil {
			return args, err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	return args, nil
}

// submitTransaction is a helper function that submits tx to txPool and creates a log entry.
func submitTransaction(ctx context.Context, b Backend, tx *types.Transaction, signature []byte) (common.Hash, error) {
	signedTx, err := tx.WithSignature(signature)
	if err != nil {
		return common.Hash{}, err
	}

	if err := b.SendTx(ctx, signedTx); err != nil {
		return common.Hash{}, err
	}

	if signedTx.To() == nil {
		from, _ := signedTx.From()
		addr := crypto.CreateAddress(from, signedTx.Nonce())
		glog.V(logger.Info).Infof("Tx(%s) created: %s\n", signedTx.Hash().Hex(), addr.Hex())
	} else {
		glog.V(logger.Info).Infof("Tx(%s) to: %s\n", signedTx.Hash().Hex(), tx.To().Hex())
	}

	return signedTx.Hash(), nil
}

// SendTransaction creates a transaction for the given argument, sign it and submit it to the
// transaction pool.
func (s *PublicTransactionPoolAPI) SendTransaction(ctx context.Context, args SendTxArgs) (common.Hash, error) {
	var err error
	args, err = prepareSendTxArgs(ctx, args, s.b)
	if err != nil {
		return common.Hash{}, err
	}

	if args.Nonce == nil {
		nonce, err := s.b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return common.Hash{}, err
		}
		args.Nonce = hexUintPointer(nonce)
	}

	var tx *types.Transaction
	if args.To == nil {
		tx = types.NewContractCreation(uint64(*args.Nonce), args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	} else {
		tx = types.NewTransaction(uint64(*args.Nonce), *args.To, args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	}

	signature, err := s.b.AccountManager().Sign(args.From, tx.SigHash().Bytes())
	if err != nil {
		return common.Hash{}, err
	}

	return submitTransaction(ctx, s.b, tx, signature)
}

// SendRawTransaction will add the signed transaction to the transaction pool.
// The sender is responsible for signing the transaction and using the correct nonce.
func (s *PublicTransactionPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (string, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return "", err
	}

	if err := s.b.SendTx(ctx, tx); err != nil {
		return "", err
	}

	if tx.To() == nil {
		from, err := tx.FromFrontier()
		if err != nil {
			return "", err
		}
		addr := crypto.CreateAddress(from, tx.Nonce())
		glog.V(logger.Info).Infof("Tx(%x) created: %x\n", tx.Hash(), addr)
	} else {
		glog.V(logger.Info).Infof("Tx(%x) to: %x\n", tx.Hash(), tx.To())
	}

	return tx.Hash().Hex(), nil
}

// Sign signs the given hash using the key that matches the address. The key must be
// unlocked in order to sign the hash.
func (s *PublicTransactionPoolAPI) Sign(addr common.Address, hash common.Hash) (string, error) {
	signature, error := s.b.AccountManager().Sign(addr, hash[:])
	return common.ToHex(signature), error
}

// SignTransactionArgs represents the arguments to sign a transaction.
type SignTransactionArgs struct {
	From     common.Address
	To       *common.Address
	Nonce    *hexutil.Uint
	Value    *hexutil.Big
	Gas      *hexutil.Big
	GasPrice *hexutil.Big
	Data     hexutil.Bytes

	BlockNumber int64
}

// Tx is a helper object for argument and return values
type Tx struct {
	tx *types.Transaction

	To       *common.Address `json:"to"`
	From     common.Address  `json:"from"`
	Nonce    *hexutil.Uint   `json:"nonce"`
	Value    *hexutil.Big    `json:"value"`
	Data     hexutil.Bytes   `json:"data"`
	GasLimit *hexutil.Big    `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Hash     common.Hash     `json:"hash"`
}

// UnmarshalJSON parses JSON data into tx.
func (tx *Tx) UnmarshalJSON(b []byte) (err error) {
	type txFields Tx
	var req txFields
	if err := json.Unmarshal(b, &req); err != nil {
		return err
	}
	if req.Nonce == nil {
		return fmt.Errorf("need nonce")
	}
	*tx = Tx(req)

	// Apply defaults.
	if tx.Value == nil {
		tx.Value = new(hexutil.Big)
	}
	if tx.GasLimit == nil {
		tx.GasLimit = defaultGas
	}
	if tx.GasPrice == nil {
		tx.GasPrice = (*hexutil.Big)(big.NewInt(int64(50000000000)))
	}

	if req.To == nil {
		tx.tx = types.NewContractCreation(uint64(*tx.Nonce), tx.Value.ToInt(), tx.GasLimit.ToInt(), tx.GasPrice.ToInt(), tx.Data)
	} else {
		tx.tx = types.NewTransaction(uint64(*tx.Nonce), *tx.To, tx.Value.ToInt(), tx.GasLimit.ToInt(), tx.GasPrice.ToInt(), tx.Data)
	}
	return nil
}

// SignTransactionResult represents a RLP encoded signed transaction.
type SignTransactionResult struct {
	Raw hexutil.Bytes `json:"raw"`
	Tx  *Tx           `json:"tx"`
}

func newTx(t *types.Transaction) *Tx {
	from, _ := t.FromFrontier()
	return &Tx{
		tx:       t,
		To:       t.To(),
		From:     from,
		Value:    (*hexutil.Big)(t.Value()),
		Nonce:    hexUintPointer(t.Nonce()),
		Data:     hexutil.Bytes(t.Data()),
		GasLimit: (*hexutil.Big)(t.Gas()),
		GasPrice: (*hexutil.Big)(t.GasPrice()),
		Hash:     t.Hash(),
	}
}

// SignTransaction will sign the given transaction with the from account.
// The node needs to have the private key of the account corresponding with
// the given from address and it needs to be unlocked.
func (s *PublicTransactionPoolAPI) SignTransaction(ctx context.Context, args SignTransactionArgs) (*SignTransactionResult, error) {
	if args.Gas == nil {
		args.Gas = defaultGas
	}
	if args.GasPrice == nil {
		price, err := s.b.SuggestPrice(ctx)
		if err != nil {
			return nil, err
		}
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}

	if args.Nonce == nil {
		nonce, err := s.b.GetPoolNonce(ctx, args.From)
		if err != nil {
			return nil, err
		}
		args.Nonce = hexUintPointer(nonce)
	}

	var tx *types.Transaction
	if args.To == nil {
		tx = types.NewContractCreation(uint64(*args.Nonce), args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	} else {
		tx = types.NewTransaction(uint64(*args.Nonce), *args.To, args.Value.ToInt(), args.Gas.ToInt(), args.GasPrice.ToInt(), args.Data)
	}

	signedTx, err := s.sign(args.From, tx)
	if err != nil {
		return nil, err
	}
	rlpTx, err := rlp.EncodeToBytes(signedTx)
	if err != nil {
		return nil, err
	}
	return &SignTransactionResult{rlpTx, newTx(signedTx)}, nil
}

// PendingTransactions returns the transactions that are in the transaction pool and have a from address that is one of
// the accounts this node manages.
func (s *PublicTransactionPoolAPI) PendingTransactions() []*RPCTransaction {
	pending := s.b.GetPoolTransactions()
	transactions := make([]*RPCTransaction, 0, len(pending))
	for _, tx := range pending {
		from, _ := tx.FromFrontier()
		if s.b.AccountManager().HasAddress(from) {
			transactions = append(transactions, newRPCPendingTransaction(tx))
		}
	}
	return transactions
}

// Resend accepts an existing transaction and a new gas price and limit. It will remove the given transaction from the
// pool and reinsert it with the new gas price and limit.
func (s *PublicTransactionPoolAPI) Resend(ctx context.Context, tx *Tx, gasPrice, gasLimit *hexutil.Big) (common.Hash, error) {

	pending := s.b.GetPoolTransactions()
	for _, p := range pending {
		if pFrom, err := p.FromFrontier(); err == nil && pFrom == tx.From && p.SigHash() == tx.tx.SigHash() {
			if gasPrice == nil {
				gasPrice = (*hexutil.Big)(tx.tx.GasPrice())
			}
			if gasLimit == nil {
				gasLimit = (*hexutil.Big)(tx.tx.Gas())
			}

			var newTx *types.Transaction
			if tx.tx.To() == nil {
				newTx = types.NewContractCreation(tx.tx.Nonce(), tx.tx.Value(), gasPrice.ToInt(), gasLimit.ToInt(), tx.tx.Data())
			} else {
				newTx = types.NewTransaction(tx.tx.Nonce(), *tx.tx.To(), tx.tx.Value(), gasPrice.ToInt(), gasLimit.ToInt(), tx.tx.Data())
			}

			signedTx, err := s.sign(tx.From, newTx)
			if err != nil {
				return common.Hash{}, err
			}

			s.b.RemoveTx(tx.Hash)
			if err = s.b.SendTx(ctx, signedTx); err != nil {
				return common.Hash{}, err
			}

			return signedTx.Hash(), nil
		}
	}

	return common.Hash{}, fmt.Errorf("Transaction %#x not found", tx.Hash)
}

// PublicDebugAPI is the collection of Etheruem APIs exposed over the public
// debugging endpoint.
type PublicDebugAPI struct {
	b Backend
}

// NewPublicDebugAPI creates a new API definition for the public debug methods
// of the Ethereum service.
func NewPublicDebugAPI(b Backend) *PublicDebugAPI {
	return &PublicDebugAPI{b: b}
}

// GetBlockRlp retrieves the RLP encoded for of a single block.
func (api *PublicDebugAPI) GetBlockRlp(ctx context.Context, number rpc.BlockNumber) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, number)
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	encoded, err := rlp.EncodeToBytes(block)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", encoded), nil
}

// PrintBlock retrieves a block and returns its pretty printed form.
func (api *PublicDebugAPI) PrintBlock(ctx context.Context, number rpc.BlockNumber) (string, error) {
	block, _ := api.b.BlockByNumber(ctx, number)
	if block == nil {
		return "", fmt.Errorf("block #%d not found", number)
	}
	return block.String(), nil
}

// SeedHash retrieves the seed hash of a block.
func (api *PublicDebugAPI) SeedHash(ctx context.Context, number hexutil.Uint) (hexutil.Bytes, error) {
	hash, err := ethash.GetSeedHash(uint64(number))
	if err != nil {
		return nil, err
	}
	return hash, nil
}

// PrivateDebugAPI is the collection of Etheruem APIs exposed over the private
// debugging endpoint.
type PrivateDebugAPI struct {
	b Backend
}

// NewPrivateDebugAPI creates a new API definition for the private debug methods
// of the Ethereum service.
func NewPrivateDebugAPI(b Backend) *PrivateDebugAPI {
	return &PrivateDebugAPI{b: b}
}

// ChaindbProperty returns leveldb properties of the chain database.
func (api *PrivateDebugAPI) ChaindbProperty(property string) (string, error) {
	ldb, ok := api.b.ChainDb().(interface {
		LDB() *leveldb.DB
	})
	if !ok {
		return "", fmt.Errorf("chaindbProperty does not work for memory databases")
	}
	if property == "" {
		property = "leveldb.stats"
	} else if !strings.HasPrefix(property, "leveldb.") {
		property = "leveldb." + property
	}
	return ldb.LDB().GetProperty(property)
}

// SetHead rewinds the head of the blockchain to a previous block.
func (api *PrivateDebugAPI) SetHead(number uint64) {
	api.b.SetHead(number)
}

// PublicNetAPI offers network related RPC methods
type PublicNetAPI struct {
	net            *p2p.Server
	networkVersion int
}

// NewPublicNetAPI creates a new net API instance.
func NewPublicNetAPI(net *p2p.Server, networkVersion int) *PublicNetAPI {
	return &PublicNetAPI{net, networkVersion}
}

// Listening returns an indication if the node is listening for network connections.
func (s *PublicNetAPI) Listening() bool {
	return true // always listening
}

// PeerCount returns the number of connected peers
func (s *PublicNetAPI) PeerCount() hexutil.Uint {
	return hexutil.Uint(uint64(s.net.PeerCount()))
}

// Version returns the current ethereum protocol version.
func (s *PublicNetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}

func hexUintPointer(i uint64) *hexutil.Uint {
	return (*hexutil.Uint)(&i)
}
