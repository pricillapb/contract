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

// Command newblocks prints the latest block known to a remote node.
package main

import (
	"log"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

type Block struct {
	Number string
	Hash   common.Hash
}

func (b *Block) BigNumber() *big.Int {
	n, _ := new(big.Int).SetString(b.Number, 0)
	return n
}

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("Usage: newblocks <url>")
	}
	client, err := rpc.Dial(os.Args[1])
	if err != nil {
		log.Fatalln("can't connect:", err)
	}

	// Shut down with Ctrl-C.
	go func() {
		interrupt := make(chan os.Signal, 1)
		signal.Notify(interrupt, os.Interrupt)
		<-interrupt
		log.Println("bye")
		client.Close()
	}()

	for {
		subch, sub := subscribe(client)

		// In the inner loop, we track subscription events.
		for event := range subch {
			log.Println("latest block:", event.BigNumber(), event.Hash.Hex())
		}
		// The subscription channel has been closed.
		// Stop reconnecting if the close was intentional (e.g. client.Close was called).
		if sub.Err() == nil {
			return
		}
		log.Println("connection lost:", sub.Err())
	}
}

func subscribe(client *rpc.Client) (chan Block, *rpc.ClientSubscription) {
	var lastBlock Block
	for {
		subch := make(chan Block)
		sub, err := client.EthSubscribe(subch, "newBlocks", map[string]interface{}{})
		if err != nil {
			log.Println("subscribe error:", err)
			goto backoff
		}
		// The connection is established now.
		// Re-sync with the current state.
		if err := client.Call(&lastBlock, "eth_getBlockByNumber", "latest", false); err != nil {
			log.Println("can't get latest block:", err)
			goto backoff
		}
		log.Println("connected: current block is", lastBlock.BigNumber(), lastBlock.Hash.Hex())
		return subch, sub

	backoff:
		time.Sleep(2 * time.Second)
	}
}
