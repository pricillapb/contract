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

// package filters implements an ethereum filtering system for block,
// transactions and log events.
package filters

import (
	"sync"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
)

// FilterSystem manages filters that filter specific events such as
// block, transaction and log events. The Filtering system can be used to listen
// for specific LOG events fired by the EVM (Ethereum Virtual Machine).
type FilterSystem struct {
	sub         event.Subscription
	add, remove chan *Filter
	closed      chan struct{}

	mu  sync.RWMutex
	all map[string]*Filter // all known filters
}

// NewFilterSystem returns a newly allocated filter manager
func NewFilterSystem(mux *event.TypeMux) *FilterSystem {
	fs := &FilterSystem{
		all:    make(map[string]*Filter),
		closed: make(chan struct{}),
		add:    make(chan *Filter),
		remove: make(chan *Filter),
		sub: mux.Subscribe(
			core.PendingLogsEvent{},
			core.RemovedLogsEvent{},
			core.ChainEvent{},
			core.TxPreEvent{},
			vm.Logs(nil),
		),
	}
	go fs.filterLoop()
	return fs
}

// Stop quits the filter loop required for polling events
func (fs *FilterSystem) Stop() {
	fs.sub.Unsubscribe()
	<-fs.closed
}

// Add starts sending updates to the given filter.
func (fs *FilterSystem) Add(f *Filter) bool {
	select {
	case fs.add <- f:
		return true
	case <-fs.closed:
		return false
	}
}

// Remove removes a filter. The filter will not receive
// any updates after Remove has returned.
func (fs *FilterSystem) Remove(f *Filter) {
	select {
	case fs.remove <- f:
	case <-fs.closed:
	}
}

func (fs *FilterSystem) Get(id string) *Filter {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return fs.all[id]
}

type filterIndex map[FilterType]map[string]*Filter

// filterLoop waits for specific events from ethereum and fires their handlers
// when the filter matches the requirements.
func (fs *FilterSystem) filterLoop() {
	index := make(filterIndex) // per-event index of filters

	defer close(fs.closed)
	for {
		select {
		case event, ok := <-fs.sub.Chan():
			if !ok {
				return // event mux closed
			}
			deliver(index, event)

		case f := <-fs.add:
			// Lazily initialise the index for this filter type.
			if index[f.typ] == nil {
				index[f.typ] = make(map[string]*Filter)
			}
			// Add it to the index. We need to lock for that
			// because Get reads from the same map.
			fs.mu.Lock()
			if _, found := fs.all[f.ID()]; !found {
				fs.all[f.ID()] = f
				index[f.typ][f.ID()] = f
			}
			fs.mu.Unlock()

		case f := <-fs.remove:
			if index[f.typ] != nil {
				delete(index[f.typ], f.id)
			}
			fs.mu.Lock()
			delete(fs.all, f.ID())
			fs.mu.Unlock()
		}
	}
}

func deliver(index filterIndex, event *event.Event) {
	iterFilters := func(typ FilterType, cb func(*Filter)) {
		for _, f := range index[typ] {
			if !f.created.After(event.Time) {
				cb(f)
			}
		}
	}

	switch ev := event.Data.(type) {
	case core.ChainEvent:
		iterFilters(ChainFilter, func(f *Filter) {
			f.BlockCallback(ev.Block, ev.Logs)
		})
	case core.TxPreEvent:
		iterFilters(PendingTxFilter, func(f *Filter) {
			f.TransactionCallback(ev.Tx)
		})
	case vm.Logs:
		iterFilters(LogFilter, func(f *Filter) {
			for _, log := range f.FilterLogs(ev) {
				f.LogCallback(log, false)
			}
		})
	case core.RemovedLogsEvent:
		iterFilters(LogFilter, func(f *Filter) {
			for _, log := range f.FilterLogs(ev.Logs) {
				f.LogCallback(log, true)
			}
		})
	case core.PendingLogsEvent:
		iterFilters(PendingLogFilter, func(f *Filter) {
			for _, log := range f.FilterLogs(ev.Logs) {
				f.LogCallback(log, false)
			}
		})
	}
}
