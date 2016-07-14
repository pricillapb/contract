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
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatalln("Usage: newblocks <url>")
	}
	client, err := rpc.Dial(os.Args[1])
	if err != nil {
		log.Fatalln("can't connect:", err)
	}

	var (
		ec     = ethclient{Client: client}
		blocks = make(chan Block)
		stats  = make(chan BasicStats)
	)
	go ec.subscribeBlocks(blocks)
	go ec.pollStats(stats)
	for {
		select {
		case event := <-blocks:
			log.Println("latest block:", event.Number, event.Hash.Hex())
		case stats := <-stats:
			log.Printf("latest stats: %v", stats)
		}
	}
}

type ethclient struct {
	*rpc.Client
}

type Block struct {
	Number *rpc.HexNumber
	Hash   common.Hash
}

type BasicStats struct {
	mining   bool
	numPeers *rpc.HexNumber
	hashrate *rpc.HexNumber
	gasPrice *rpc.HexNumber
}

func (c *ethclient) subscribeBlocks(ch chan Block) {
	var lastBlock Block
	for {
		sub, err := c.EthSubscribe(ch, "newBlocks", map[string]interface{}{})
		if err != nil {
			log.Println("subscribe error:", err)
			goto backoff
		}
		// The connection is established now.
		// Re-sync with the current state.
		if err := c.Call(&lastBlock, "eth_getBlockByNumber", "latest", false); err != nil {
			sub.Unsubscribe()
			log.Println("can't get last block:", err)
			goto backoff
		}
		ch <- lastBlock

		// Wait for the subscription to go down.
		if err = <-sub.Err(); err == nil {
			return
		} else if err != nil {
			log.Println("connection lost:", err)
			goto backoff
		}
		continue

	backoff:
		time.Sleep(2 * time.Second)
	}
}

func (c *ethclient) pollStats(ch chan BasicStats) {
	for {
		var stats BasicStats
		err := c.BatchCall([]rpc.BatchElem{
			{Method: "net_peerCount", Result: &stats.numPeers},
			{Method: "eth_mining", Result: &stats.mining},
			{Method: "eth_hashrate", Result: &stats.hashrate},
			{Method: "eth_gasPrice", Result: &stats.gasPrice},
		})
		if err == rpc.ErrClientQuit {
			return
		} else if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		ch <- stats
		time.Sleep(5 * time.Second)
	}
}
