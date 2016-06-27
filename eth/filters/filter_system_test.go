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

package filters

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
)

func TestCallbacks(t *testing.T) {
	var (
		mux            event.TypeMux
		fs             = NewFilterSystem(&mux)
		blockDone      = make(chan struct{})
		txDone         = make(chan struct{})
		logDone        = make(chan struct{})
		removedLogDone = make(chan struct{})
		pendingLogDone = make(chan struct{})
	)

	blockFilter := New(ChainFilter, nil)
	blockFilter.BlockCallback = func(*types.Block, vm.Logs) {
		close(blockDone)
	}
	txFilter := New(PendingTxFilter, nil)
	txFilter.TransactionCallback = func(*types.Transaction) {
		close(txDone)
	}
	logFilter := New(LogFilter, nil)
	logFilter.LogCallback = func(l *vm.Log, removed bool) {
		if !removed {
			close(logDone)
		}
	}
	removedLogFilter := New(LogFilter, nil)
	removedLogFilter.LogCallback = func(l *vm.Log, removed bool) {
		if removed {
			close(removedLogDone)
		}
	}
	pendingLogFilter := New(PendingLogFilter, nil)
	pendingLogFilter.LogCallback = func(*vm.Log, bool) {
		close(pendingLogDone)
	}

	fs.Add(blockFilter)
	fs.Add(txFilter)
	fs.Add(logFilter)
	fs.Add(removedLogFilter)
	fs.Add(pendingLogFilter)

	mux.Post(core.ChainEvent{})
	mux.Post(core.TxPreEvent{})
	mux.Post(vm.Logs{&vm.Log{}})
	mux.Post(core.RemovedLogsEvent{Logs: vm.Logs{&vm.Log{}}})
	mux.Post(core.PendingLogsEvent{Logs: vm.Logs{&vm.Log{}}})

	const dura = 5 * time.Second
	failTimer := time.NewTimer(dura)
	select {
	case <-blockDone:
	case <-failTimer.C:
		t.Error("block filter failed to trigger (timeout)")
	}

	failTimer.Reset(dura)
	select {
	case <-txDone:
	case <-failTimer.C:
		t.Error("transaction filter failed to trigger (timeout)")
	}

	failTimer.Reset(dura)
	select {
	case <-logDone:
	case <-failTimer.C:
		t.Error("log filter failed to trigger (timeout)")
	}

	failTimer.Reset(dura)
	select {
	case <-removedLogDone:
	case <-failTimer.C:
		t.Error("removed log filter failed to trigger (timeout)")
	}

	failTimer.Reset(dura)
	select {
	case <-pendingLogDone:
	case <-failTimer.C:
		t.Error("pending log filter failed to trigger (timeout)")
	}
}
