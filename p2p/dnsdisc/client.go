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
	"bytes"
	"context"
	"crypto/ecdsa"
	"fmt"
	"net"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
)

const defaultTimeout = 5 * time.Second

// Resolver is a DNS resolver that can query TXT records.
type Resolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

// Client discovers nodes by querying DNS servers.
type Client struct {
	trees    map[string]*Tree
	resolver Resolver
}

// NewClient creates a client. If resolver is nil, the default DNS resolver is used.
func NewClient(resolver Resolver, urls ...string) (*Client, error) {
	if resolver == nil {
		resolver = new(net.Resolver)
	}
	c := &Client{
		resolver: resolver,
		trees:    make(map[string]*Tree),
	}
	for _, url := range urls {
		if err := c.AddTree(url); err != nil {
			return nil, err
		}
	}
	return c, nil
}

// AddTree adds a enrtree:// URL to crawl.
func (c *Client) AddTree(url string) error {
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

// SyncTree downloads the entire node tree at the given URL. This doesn't add the tree for
// later use, but any previously-synced entries are reused.
func (c *Client) SyncTree(url string) (*Tree, error) {
	le, err := parseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid enrtree URL: %v", err)
	}
	var t *Tree
	if existing, ok := c.trees[le.domain]; ok && keysEqual(existing.location.pubkey, le.pubkey) {
		t = existing
	} else {
		t = newTreeAt(le)
	}
	err = c.syncTree(t)
	return t, err
}

// RandomNode returns a random node from any tree added to the client.
func (c *Client) RandomNode(ctx context.Context) *enode.Node {
	return nil
}

func (c *Client) syncTree(t *Tree) error {
	if err := c.updateRoot(t); err != nil {
		return err
	}
	for len(t.missing) > 0 {
		hash := t.missing[0]
		t.missing = t.missing[1:]

		if _, ok := t.entries[hash]; ok {
			continue // branch available locally, skip sync
		}
		ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
		e, err := c.resolveEntry(ctx, t.location.domain, hash)
		cancel()
		switch e := e.(type) {
		case enrEntry, linkEntry:
			t.entries[hash] = e
		case subtreeEntry:
			t.entries[hash] = e
			t.missing = append(t.missing, e.children...)
		default:
			return err
		}
	}
	return nil
}

// updateRoot ensures that the given tree has an up-to-date root.
func (c *Client) updateRoot(t *Tree) error {
	if t.root != nil {
		// TODO: Implement last update threshold. If last update too long ago, re-fetch. If
		// re-fetch doesn't work (i.e. we're offline), use the existing root.
		return nil
	}
	t.lastUpdate = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	root, err := c.resolveRoot(ctx, t.location)
	if err != nil {
		return err
	}
	t.root = &root
	t.missing = []string{root.hash}
	return nil
}

func (c *Client) resolveRoot(ctx context.Context, loc linkEntry) (rootEntry, error) {
	txts, err := c.resolver.LookupTXT(ctx, loc.domain)
	if err != nil {
		return rootEntry{}, err
	}
	for _, txt := range txts {
		if e, err := parseRoot(txt); err == nil {
			if !e.verifySignature(loc.pubkey) {
				return e, fmt.Errorf("invalid signature")
			}
			return e, nil
		}
	}
	return rootEntry{}, fmt.Errorf("no root found at %q", loc.domain)
}

func (c *Client) resolveEntry(ctx context.Context, domain, hash string) (entry, error) {
	wantHash, err := b32format.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid base32 hash")
	}
	txts, err := c.resolver.LookupTXT(ctx, hash+"."+domain)
	if err != nil {
		return nil, err
	}
	for _, txt := range txts {
		if e, err := parseEntry(txt); err == nil {
			if !bytes.HasPrefix(crypto.Keccak256([]byte(txt)), wantHash) {
				err = fmt.Errorf("hash mismatch at %s.%s", hash, domain)
			}
			return e, err
		}
	}
	return nil, err
}

func keysEqual(k1, k2 *ecdsa.PublicKey) bool {
	return k1.Curve == k2.Curve && k1.X.Cmp(k2.X) == 0 && k1.Y.Cmp(k2.Y) == 0
}
