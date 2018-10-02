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

// Package dnsdisc implements node discovery via DNS (EIP-1459).
package dnsdisc

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"
)

// Resolver is a DNS resolver that can query TXT records.
type Resolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

// Crawler discovers nodes by querying DNS domains.
type Crawler struct {
	trees    map[string]*Tree
	resolver Resolver
}

// NewCrawler creates a new crawler. If resolver is nil, the default DNS resolver is used.
func NewCrawler(resolver Resolver, urls ...string) (*Crawler, error) {
	if resolver == nil {
		resolver = new(net.Resolver)
	}
	c := &Crawler{resolver: resolver}
	for _, url := range urls {
		if err := c.AddTree(url); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// AddTree adds a enrtree:// URL to crawl.
func (c *Crawler) AddTree(url string) error {
	le, err := parseURL(url)
	if err != nil {
		return fmt.Errorf("invalid enrtree URL: %v", err)
	}
	if existing, ok := c.trees[le.domain]; ok && !keysEqual(existing.location.pubkey, le.pubkey) {
		return fmt.Errorf("conflicting public keys for domain %q", le.domain)
	}
	c.trees[le.domain] = newTreeAt(le)
	return nil
}

func keysEqual(k1, k2 *ecdsa.PublicKey) bool {
	return k1.Curve == k2.Curve && k1.X.Cmp(k2.X) == 0 && k1.Y.Cmp(k2.Y) == 0
}
