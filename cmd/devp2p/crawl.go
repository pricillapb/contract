// Copyright 2019 The go-ethereum Authors
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

package main

import (
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"github.com/ethereum/go-ethereum/p2p/discutil"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

type crawler struct {
	input     nodeSet
	output    nodeSet
	disc      *discover.UDPv4
	iters     []discutil.Iterator
	inputIter discutil.Iterator
	ch        chan *enode.Node
	closed    chan struct{}

	// settings
	revalidateInterval time.Duration
}

func newCrawler(input nodeSet, disc *discover.UDPv4, iters ...discutil.Iterator) *crawler {
	c := &crawler{
		input:     input,
		output:    make(nodeSet, len(input)),
		disc:      disc,
		iters:     iters,
		inputIter: discutil.IterNodes(input.nodes()),
		ch:        make(chan *enode.Node),
		closed:    make(chan struct{}),
	}
	c.iters = append(c.iters, c.inputIter)
	// Copy input to output initially. Any nodes that fail validation
	// will be dropped from output during the run.
	for id, n := range input {
		c.output[id] = n
	}
	return c
}

func (c *crawler) run(timeout time.Duration) nodeSet {
	var (
		timeoutTimer = time.NewTimer(timeout)
		timeoutCh    <-chan time.Time
		doneCh       = make(chan discutil.Iterator, len(c.iters))
		liveIters    = len(c.iters)
	)
	for _, it := range c.iters {
		go c.runIterator(doneCh, it)
	}

loop:
	for {
		select {
		case n := <-c.ch:
			c.updateNode(n)
		case it := <-doneCh:
			liveIters--
			if liveIters == 0 {
				break loop
			}
			if it == c.inputIter {
				// Enable timeout when we're done revalidating the input nodes.
				log.Info("Revalidation of input set is done", "len", len(c.input))
				if timeout > 0 {
					timeoutCh = timeoutTimer.C
				}
			}
		case <-timeoutCh:
			break loop
		}
	}

	close(c.closed)
	for _, it := range c.iters {
		it.Close()
	}
	for ; liveIters > 0; liveIters-- {
		<-doneCh
	}
	return c.output
}

func (c *crawler) runIterator(done chan<- discutil.Iterator, it discutil.Iterator) {
	defer func() { done <- it }()
	for it.Next() {
		select {
		case c.ch <- it.Node():
		case <-c.closed:
			return
		}
	}
}

func (c *crawler) updateNode(n *enode.Node) {
	existing, ok := c.output[n.ID()]

	// Skip validation of recently-seen nodes.
	if ok && time.Since(existing.LastSeen) < c.revalidateInterval {
		return
	}

	// Request the node record.
	nn, err := c.disc.RequestENR(n)
	if err != nil {
		if existing.Checks == 0 {
			log.Debug("Skipping node", "id", n.ID())
			return
		}
		existing.Checks /= 2
	} else {
		if !ok {
			existing.FirstSeen = truncNow()
		}
		existing.N = nn
		existing.Seq = nn.Seq()
		existing.LastSeen = truncNow()
		existing.Checks++
	}

	// Store/update node in output set.
	if existing.Checks <= 0 {
		log.Info("Removing node", "id", n.ID())
		delete(c.output, n.ID())
	} else {
		log.Info("Updating node", "id", n.ID(), "seq", existing.Seq, "checks", existing.Checks)
		c.output[n.ID()] = existing
	}
}

func truncNow() time.Time {
	return time.Now().UTC().Truncate(1 * time.Second)
}
