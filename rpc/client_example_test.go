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

package rpc_test

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/rpc"
)

// In this example, our client whishes to track the latest 'block number'
// known to the server. The server supports two methods:
//
// eth_getBlockByNumber("latest", {})
//    returns the latest block object.
//
// eth_subscribe("newBlocks")
//    creates a subscription which fires block objects when new blocks arrive.

type Block struct {
	Number *big.Int
}

func ExampleClientSubscription() {
	// Create the client.
	client, _ := rpc.Dial("ws://127.0.0.1:8485")

	for {
		// Subscribe to new blocks.
		subch := make(chan Block)
		sub, err := client.EthSubscribe(subch, "newBlocks", map[string]interface{}{})
		if err != nil {
			fmt.Println("subscribe error:", err)
			continue
		}
		// The connection is established now.
		// Re-sync with the current state.
		var lastBlock Block
		if err := client.Call(&lastBlock, "eth_getBlockByNumber", "latest", false); err != nil {
			fmt.Println("can't get latest block:", err)
			continue
		}
		fmt.Println("connected: current block is", lastBlock.Number)

		// In the inner loop, we track subscription events.
		for block := range subch {
			fmt.Println("latest block:", block.Number)
		}
		// The subscription channel has been closed.
		// Stop reconnecting if the close was intentional (e.g. client.Close was called).
		if sub.Err() == nil {
			return
		}
	}
}
