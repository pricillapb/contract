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
	"crypto/ecdsa"
	"encoding/base32"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/ethereum/go-ethereum/rlp"
	"golang.org/x/crypto/sha3"
)

type TXT struct {
	Name, Content string
}

// Tree is a merkle tree of node records.
type Tree struct {
	root    *rootEntry
	entries map[string]entry
}

// Sign signs the tree with the given private key and sets the update sequence number.
func (t *Tree) Sign(key *ecdsa.PrivateKey, seq uint, domain string) (string, error) {
	root := *t.root
	root.seq = seq
	sig, err := crypto.Sign(root.sigHash(), key)
	if err != nil {
		return "", err
	}
	root.sig = sig
	t.root = &root
	link := &linkEntry{domain, &key.PublicKey}
	return link.url(), nil
}

// Seq returns the update sequence number of the tree.
func (t *Tree) Seq() uint {
	return t.root.seq
}

// ToTXT returns all DNS TXT records required for the tree.
func (t *Tree) ToTXT(domain string) []TXT {
	records := []TXT{
		{domain, t.root.String()},
	}
	for _, e := range t.entries {
		sd := subdomain(e)
		if domain != "" {
			sd = domain + "." + sd
		}
		records = append(records, TXT{sd, e.String()})
	}
	return records
}

var (
	hashAbbrev  = 16
	maxChildren = 300 / (hashAbbrev * (13 / 8))
)

// MakeTree creates a tree containing the given nodes and links.
func MakeTree(nodes []*enode.Node, links []string) (*Tree, error) {
	// Sort records by ID.
	records := make([]*enr.Record, len(nodes))
	for i := range nodes {
		records[i] = nodes[i].Record()
	}
	sort.Slice(records, func(i, j int) bool {
		id1, id2 := nodes[i].ID(), nodes[j].ID()
		return bytes.Compare(id1[:], id2[:]) < 0
	})

	// Create the leaf list.
	enrNodes := make([]entry, len(records))
	for i, r := range records {
		enrNodes[i] = &enrEntry{r}
	}
	linkNodes := make([]entry, len(links))
	for i, l := range links {
		le, err := parseURL(l)
		if err != nil {
			return nil, err
		}
		linkNodes[i] = le
	}

	// Create intermediate nodes.
	t := &Tree{entries: make(map[string]entry)}
	eroot := t.build(enrNodes)
	t.entries[subdomain(eroot)] = eroot
	lroot := t.build(linkNodes)
	t.entries[subdomain(lroot)] = lroot
	t.root = &rootEntry{eroot: subdomain(eroot), lroot: subdomain(lroot)}
	return t, nil
}

func (t *Tree) build(entries []entry) entry {
	if len(entries) == 1 {
		return entries[0]
	}
	if len(entries) < maxChildren {
		hashes := make([]string, len(entries))
		for i, e := range entries {
			hashes[i] = subdomain(e)
			t.entries[hashes[i]] = e
		}
		return &subtreeEntry{hashes}
	}
	var roots []entry
	for len(entries) > 0 {
		n := maxChildren
		if len(entries) < n {
			n = len(entries)
		}
		e := t.build(entries[:n])
		roots = append(roots, e)
		t.entries[subdomain(e)] = e
	}
	return t.build(roots)
}

// Entry Types

type entry interface {
	fmt.Stringer
}

type (
	rootEntry struct {
		eroot string
		lroot string
		seq   uint
		sig   []byte
	}
	subtreeEntry struct {
		children []string
	}
	enrEntry struct {
		record *enr.Record
	}
	linkEntry struct {
		domain string
		pubkey *ecdsa.PublicKey
	}
)

// Entry Encoding

var (
	b32format = base32.StdEncoding.WithPadding(base32.NoPadding)
	b64format = base64.URLEncoding
)

func subdomain(e entry) string {
	h := sha3.NewLegacyKeccak256()
	io.WriteString(h, e.String())
	return b32format.EncodeToString(h.Sum(nil)[:16])
}

func (e *rootEntry) String() string {
	return fmt.Sprintf(rootPrefix+" e=%s l=%s seq=%d sig=%s", e.eroot, e.lroot, e.seq, b64format.EncodeToString(e.sig))
}

func (e *rootEntry) sigHash() []byte {
	h := sha3.NewLegacyKeccak256()
	fmt.Fprintf(h, rootPrefix+" e=%s l=%s seq=%d", e.eroot, e.lroot, e.seq)
	return h.Sum(nil)
}

func (e *rootEntry) verifySignature(pubkey *ecdsa.PublicKey) bool {
	sig := e.sig[:len(e.sig)-1] // remove recovery id
	return crypto.VerifySignature(crypto.FromECDSAPub(pubkey), e.sigHash(), sig)
}

func (e *subtreeEntry) String() string {
	return "enrtree=" + strings.Join(e.children, ",")
}

func (e *enrEntry) String() string {
	enc, _ := rlp.EncodeToBytes(e.record)
	return "enr=" + b64format.EncodeToString(enc)
}

func (e *linkEntry) String() string {
	return "enrtree-link=" + e.link()
}

func (e *linkEntry) url() string {
	return "enrtree://" + e.link()
}

func (e *linkEntry) link() string {
	return fmt.Sprintf("%s@%s", b32format.EncodeToString(crypto.CompressPubkey(e.pubkey)), e.domain)
}

// Entry Parsing

var (
	errUnknownEntry = errors.New("unknown entry type")
	errEmptySubtree = errors.New("empty subtree")
	errNoPubkey     = errors.New("missing public key")
	errBadPubkey    = errors.New("invalid public key")
	errInvalidENR   = errors.New("invalid node record")
	errInvalidChild = errors.New("invalid child hash")
	errInvalidSig   = errors.New("invalid base64 signature")
	errSyntax       = errors.New("invalid syntax")
)

type entryError struct {
	typ string
	err error
}

func (err entryError) Error() string {
	return fmt.Sprintf("invalid %s entry: %v", err.typ, err.err)
}

const minHashLength = 10

func parseEntry(e string) (entry, error) {
	switch {
	case strings.HasPrefix(e, "enrtree-link="):
		return parseLink(e[13:])
	case strings.HasPrefix(e, "enrtree="):
		return parseSubtree(e[8:])
	case strings.HasPrefix(e, "enr="):
		return parseENR(e[4:])
	default:
		return nil, errUnknownEntry
	}
}

const rootPrefix = "enrtree-root=v1"

func parseRoot(e string) (rootEntry, error) {
	var eroot, lroot, sig string
	var seq uint
	if _, err := fmt.Sscanf(e, rootPrefix+" e=%s l=%s seq=%d sig=%s", &eroot, &lroot, &seq, &sig); err != nil {
		return rootEntry{}, entryError{"root", errSyntax}
	}
	if !isValidHash(eroot) || !isValidHash(lroot) {
		return rootEntry{}, entryError{"root", errInvalidChild}
	}
	sigb, err := b64format.DecodeString(sig)
	if err != nil || len(sigb) != 65 {
		return rootEntry{}, entryError{"root", errInvalidSig}
	}
	return rootEntry{eroot, lroot, seq, sigb}, nil
}

func parseLink(e string) (entry, error) {
	pos := strings.IndexByte(e, '@')
	if pos == -1 {
		return nil, entryError{"link", errNoPubkey}
	}
	keystring, domain := e[:pos], e[pos+1:]
	keybytes, err := b32format.DecodeString(keystring)
	if err != nil {
		return nil, entryError{"link", errBadPubkey}
	}
	key, err := crypto.DecompressPubkey(keybytes)
	if err != nil {
		return nil, entryError{"link", errBadPubkey}
	}
	return &linkEntry{domain, key}, nil
}

func parseSubtree(e string) (entry, error) {
	hashes := make([]string, 0, strings.Count(e, ","))
	for _, c := range strings.Split(e, ",") {
		if !isValidHash(c) {
			return nil, entryError{"subtree", errInvalidChild}
		}
		hashes = append(hashes, c)
	}
	return &subtreeEntry{hashes}, nil
}

func parseENR(e string) (entry, error) {
	enc, err := b64format.DecodeString(e)
	if err != nil {
		return nil, entryError{"enr", errInvalidENR}
	}
	var rec enr.Record
	if err := rlp.DecodeBytes(enc, &rec); err != nil {
		return nil, entryError{"enr", err}
	}
	return &enrEntry{&rec}, nil
}

func isValidHash(s string) bool {
	dlen := b32format.DecodedLen(len(s))
	if dlen < minHashLength || dlen > 32 || strings.ContainsAny(s, "\n\r") {
		return false
	}
	buf := make([]byte, 32)
	_, err := b32format.Decode(buf, []byte(s))
	return err == nil
}

// URL encoding

func parseURL(url string) (*linkEntry, error) {
	const scheme = "enrtree://"
	if !strings.HasPrefix(url, scheme) {
		return nil, fmt.Errorf("wrong/missing scheme 'enrtree'")
	}
	le, err := parseLink(url[len(scheme):])
	if err != nil {
		return nil, err.(entryError).err
	}
	return le.(*linkEntry), nil
}
