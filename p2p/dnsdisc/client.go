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

package dnsdisc

import (
	"bytes"
	"context"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
)

// Client discovers nodes by querying DNS servers.
type Client struct {
	cfg     Config
	trees   map[string]*clientTree
	entries map[string]entry // global entry cache
}

// Config holds configuration options for the client.
type Config struct {
	Timeout         time.Duration      // timeout used for DNS lookups (default 5s)
	RecheckInterval time.Duration      // time between tree root update checks (default 30min)
	ValidSchemes    enr.IdentityScheme // acceptable ENR identity schemes (default enode.ValidSchemes)
	Resolver        Resolver           // the DNS resolver to use (defaults to system DNS)
	Logger          log.Logger         // destination of client log messages (defaults to root logger)
}

// Resolver is a DNS resolver that can query TXT records.
type Resolver interface {
	LookupTXT(ctx context.Context, domain string) ([]string, error)
}

func (cfg Config) withDefaults() Config {
	const (
		defaultTimeout = 5 * time.Second
		defaultRecheck = 30 * time.Minute
	)
	if cfg.Timeout == 0 {
		cfg.Timeout = defaultTimeout
	}
	if cfg.RecheckInterval == 0 {
		cfg.RecheckInterval = defaultRecheck
	}
	if cfg.ValidSchemes == nil {
		cfg.ValidSchemes = enode.ValidSchemes
	}
	if cfg.Resolver == nil {
		cfg.Resolver = new(net.Resolver)
	}
	if cfg.Logger == nil {
		cfg.Logger = log.Root()
	}
	return cfg
}

// NewClient creates a client.
func NewClient(cfg Config, urls ...string) (*Client, error) {
	c := &Client{
		cfg:     cfg.withDefaults(),
		trees:   make(map[string]*clientTree),
		entries: make(map[string]entry),
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
	existing, ok := c.trees[le.domain]
	if ok {
		if existing.matchPubkey(le.pubkey) {
			return fmt.Errorf("conflicting public keys for domain %q", le.domain)
		}
		return nil
	}
	c.trees[le.domain] = newClientTree(c, le)
	return nil
}

// SyncTree downloads the entire node tree at the given URL. This doesn't add the tree for
// later use, but any previously-synced entries are reused.
func (c *Client) SyncTree(url string) (*Tree, error) {
	le, err := parseURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid enrtree URL: %v", err)
	}
	var ct *clientTree
	if existing, ok := c.trees[le.domain]; ok && existing.matchPubkey(le.pubkey) {
		ct = existing
	} else {
		ct = newClientTree(c, le)
	}
	if err := ct.syncAll(); err != nil {
		return nil, err
	}
	return c.collectTree(ct), nil
}

// RandomNode retrieves the next random node.
func (c *Client) RandomNode(ctx context.Context) *enode.Node {
	for {
		ct := c.randomTree()
		if ct == nil {
			return nil
		}
		n, err := ct.syncRandom(ctx)
		if err != nil {
			c.cfg.Logger.Debug("Error in DNS discovery lookup", "tree", ct.loc.domain, "err", err)
			continue
		}
		if n != nil {
			return n
		}
	}
}

// randomTree returns a random tree.
func (c *Client) randomTree() *clientTree {
	limit := rand.Intn(len(c.trees))
	for _, ct := range c.trees {
		if limit--; limit == 0 {
			return ct
		}
	}
	return nil
}

// collectTree creates a stand-alone tree from the node cache.
func (c *Client) collectTree(ct *clientTree) *Tree {
	t := &Tree{root: ct.root, entries: make(map[string]entry)}
	missing := []string{ct.links.root, ct.enrs.root}
	for len(missing) > 0 {
		e := c.entries[missing[0]]
		t.entries[missing[0]] = e
		if subtree, ok := e.(*subtreeEntry); ok {
			missing = append(missing, subtree.children...)
		}
		missing = missing[1:]
	}
	return t
}

func (c *Client) resolveRoot(ctx context.Context, loc *linkEntry) (rootEntry, error) {
	txts, err := c.cfg.Resolver.LookupTXT(ctx, loc.domain)
	c.cfg.Logger.Trace(fmt.Sprintf("DNS discovery root lookup %s", loc.domain), "err", err)
	if err != nil {
		return rootEntry{}, err
	}
	for _, txt := range txts {
		if strings.HasPrefix(txt, rootPrefix) {
			return parseAndVerifyRoot(txt, loc)
		}
	}
	return rootEntry{}, fmt.Errorf("no root found at %q", loc.domain)
}

func parseAndVerifyRoot(txt string, loc *linkEntry) (rootEntry, error) {
	e, err := parseRoot(txt)
	if err != nil {
		return e, err
	}
	if !e.verifySignature(loc.pubkey) {
		return e, entryError{"root", errInvalidSig}
	}
	return e, nil
}

func (c *Client) resolveEntry(ctx context.Context, domain, hash string) (entry, error) {
	wantHash, err := b32format.DecodeString(hash)
	if err != nil {
		return nil, fmt.Errorf("invalid base32 hash")
	}
	name := hash + "." + domain
	txts, err := c.cfg.Resolver.LookupTXT(ctx, hash+"."+domain)
	c.cfg.Logger.Trace(fmt.Sprintf("DNS discovery lookup %s", name), "err", err)
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
