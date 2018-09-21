// Copyright 2018 The go-ethereum Authors
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

package netutil

import (
	"time"

	"github.com/ethereum/go-ethereum/common/mclock"
)

// IPTracker predicts the endpoint (IP address and port) of the local host based on
// statements from other nodes.
type IPTracker struct {
	window        time.Duration
	contactWindow time.Duration
	minStatements int
	clock         mclock.Clock
	statements    map[string]ipStatement
	contact       map[string]mclock.AbsTime
}

type ipStatement struct {
	endpoint string
	time     mclock.AbsTime
}

func NewIPTracker(window, contactWindow time.Duration, minStatements int) *IPTracker {
	return &IPTracker{
		window:        window,
		contactWindow: contactWindow,
		statements:    make(map[string]ipStatement),
		minStatements: minStatements,
		contact:       make(map[string]mclock.AbsTime),
		clock:         mclock.System{},
	}
}

func (it *IPTracker) PredictFullConeNAT() bool {
	now := it.clock.Now()
	it.gcContact(now)
	it.gcStatements(now)
	for host := range it.statements {
		if _, ok := it.contact[host]; !ok {
			return true
		}
	}
	return false
}

func (it *IPTracker) PredictEndpoint() string {
	it.gcStatements(it.clock.Now())

	// Find IP with most statements.
	counts := make(map[string]int)
	maxcount, max := 0, ""
	for _, s := range it.statements {
		c := counts[s.endpoint] + 1
		counts[s.endpoint] = c
		if c > maxcount && c > it.minStatements {
			maxcount, max = c, s.endpoint
		}
	}
	return max
}

func (it *IPTracker) AddStatement(host, endpoint string) {
	it.statements[host] = ipStatement{endpoint, it.clock.Now()}
}

func (it *IPTracker) AddContact(host string) {
	it.contact[host] = it.clock.Now()
}

func (it *IPTracker) gcStatements(now mclock.AbsTime) {
	cutoff := now.Add(-it.window)
	for host, s := range it.statements {
		if s.time < cutoff {
			delete(it.statements, host)
		}
	}
}

func (it *IPTracker) gcContact(now mclock.AbsTime) {
	cutoff := now.Add(-it.contactWindow)
	for host, ct := range it.contact {
		if ct < cutoff {
			delete(it.contact, host)
		}
	}
}
