// Copyright 2019 The go-ethereum Authors
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
	"context"
	"crypto/ecdsa"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enode"
)

// clientTree is a full tree being synced.
type clientTree struct {
	c          *Client
	loc        *linkEntry
	root       *rootEntry
	lastUpdate time.Time // last revalidation of root
	links      *treeSync
	enrs       *treeSync
}

func newClientTree(c *Client, loc *linkEntry) *clientTree {
	return &clientTree{c: c, loc: loc}
}

func (ct *clientTree) matchPubkey(key *ecdsa.PublicKey) bool {
	return keysEqual(ct.loc.pubkey, key)
}

func keysEqual(k1, k2 *ecdsa.PublicKey) bool {
	return k1.Curve == k2.Curve && k1.X.Cmp(k2.X) == 0 && k1.Y.Cmp(k2.Y) == 0
}

// syncAll retrieves all entries of the tree.
func (ct *clientTree) syncAll() error {
	if err := ct.updateRoot(); err != nil {
		return err
	}
	if err := ct.links.resolveAll(); err != nil {
		return err
	}
	if err := ct.enrs.resolveAll(); err != nil {
		return err
	}
	return nil
}

// syncRandom retrieves a single entry of the tree. The Node return value
// is non-nil if the entry was a node.
func (ct *clientTree) syncRandom(ctx context.Context) (*enode.Node, error) {
	// re-check root, but don't check every call.
	if time.Since(ct.lastUpdate) > ct.c.cfg.RecheckInterval {
		if err := ct.updateRoot(); err != nil {
			return nil, err
		}
	}

	// Link tree sync has priority, run it to completion before syncing ENRs.
	if !ct.links.done() {
		hash := ct.links.missing[0]
		_, err := ct.links.resolveNext(ctx, hash)
		if err != nil {
			return nil, err
		}
		ct.links.missing = ct.links.missing[1:]
		return nil, nil
	}
	// Sync next entry in ENR tree.
	if !ct.enrs.done() {
		hash := ct.enrs.missing[0]
		e, err := ct.enrs.resolveNext(ctx, hash)
		if err != nil {
			return nil, err
		}
		ct.enrs.missing = ct.links.missing[1:]
		if ee, ok := e.(*enrEntry); ok {
			return ee.node, nil
		}
	}
	return nil, nil
}

// updateRoot ensures that the given tree has an up-to-date root.
func (ct *clientTree) updateRoot() error {
	ct.lastUpdate = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), ct.c.cfg.Timeout)
	defer cancel()
	root, err := ct.c.resolveRoot(ctx, ct.loc)
	if err != nil {
		return err
	}
	ct.root = &root
	// Invalidate subtrees if changed.
	if ct.links == nil || root.lroot != ct.links.root {
		ct.links = newTreeSync(ct.c, ct.loc, root.lroot, true)
	}
	if ct.enrs == nil || root.eroot != ct.enrs.root {
		ct.enrs = newTreeSync(ct.c, ct.loc, root.eroot, false)
	}
	return nil
}

// treeSync is the sync of an ENR or link subtree.
type treeSync struct {
	c       *Client
	loc     *linkEntry
	root    string
	missing []string // missing tree node hashes
	link    bool     // true if this sync is for the link tree
}

func newTreeSync(c *Client, loc *linkEntry, root string, link bool) *treeSync {
	return &treeSync{
		c:       c,
		root:    root,
		loc:     loc,
		link:    link,
		missing: []string{root},
	}
}

func (ts *treeSync) done() bool {
	return len(ts.missing) == 0
}

func (ts *treeSync) resolveAll() error {
	for !ts.done() {
		hash := ts.missing[0]
		ctx, cancel := context.WithTimeout(context.Background(), ts.c.cfg.Timeout)
		_, err := ts.resolveNext(ctx, hash)
		cancel()
		if err != nil {
			return err
		}
		ts.missing = ts.missing[1:]
	}
	return nil
}

func (ts *treeSync) resolveNext(ctx context.Context, hash string) (entry, error) {
	var err error
	e, ok := ts.c.entries[hash]
	if !ok {
		// branch unavailable locally, use resolver
		e, err = ts.c.resolveEntry(ctx, ts.loc.domain, hash)
		if err != nil {
			return nil, err
		}
	}
	ts.c.entries[hash] = e
	switch e := e.(type) {
	case *enrEntry:
		if ts.link {
			return nil, fmt.Errorf("found enr entry in link tree")
		}
	case *linkEntry:
		if !ts.link {
			return nil, fmt.Errorf("found link entry in enr tree")
		}
	case *subtreeEntry:
		ts.missing = append(ts.missing, e.children...)
	}
	return e, nil
}
