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

// +build none

package rpc_test

import (
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
)

// In this example, our client whishes to track the latest 'block number'
// known to the server. The server supports two methods:
//
// eth_getBlock("latest")
//    returns the latest block object.
//
// eth_subscribe("blocks")
//    creates a subscription which fires block objects when new blocks arrive.

type Block struct {
	Number *big.Int
}

type BlockNumberTracker struct {
	latestNumber *big.Int
	client       *rpc.Client
	sub          *rpc.ClientSubscription
	subch        chan Block
}

func (bt *BlockNumberTracker) reconnect() error {
	client, err := rpc.NewClient("ws://127.0.0.1:8485")
	if err != nil {
		return err
	}
	sub, err := client.EthSubscribe(bt.subch, "blocks")
	if err != nil {
		return err
	}
	bt.client, bt.sub = client, sub
	return nil
}

func ExampleClientSubscription() {
	tracker := &BlockNumberTracker{subch: make(chan Block)}

	// The outer loop manages the client connection.
	for {
		if err := tracker.reconnect(); err != nil {
			// Try reconnecting after a bit of delay.
			fmt.Println("reconnect failed:", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// The connection is established now.
		// Resync with the current state.
		if err := tracker.client.Call(&tracker.latestNumber, "eth_getBlock", "latest"); err != nil {
			fmt.Println("can't get latest block:", err)
			continue
		}

		// In the inner loop, we track subscription events.
		for event := range tracker.subch {
			fmt.Println("got event: ", err)
			tracker.latestNumber = event.Number
		}

		// The subscription channel has been closed.
		// Stop reconnecting if the close was intentional (e.g. client.Close was called).
		if tracker.sub.Err() == nil {
			return
		}
	}
}
