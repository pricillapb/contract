// Copyright 2018 The go-ethereum Authors
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
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/console"
	"github.com/ethereum/go-ethereum/p2p/dnsdisc"
	"github.com/ethereum/go-ethereum/p2p/enode"
	cli "gopkg.in/urfave/cli.v1"
)

var (
	dnsCommand = cli.Command{
		Name:  "dns",
		Usage: "DNS Discovery Commands",
		Subcommands: []cli.Command{
			dnsSyncCommand,
			dnsTXTCommand,
		},
	}
	dnsSyncCommand = cli.Command{
		Name:      "sync",
		Usage:     "Download a DNS discovery tree",
		ArgsUsage: "<url> [ <directory> ]",
		Action:    dnsSync,
		Flags:     []cli.Flag{dnsTimeoutFlag},
	}
	dnsTXTCommand = cli.Command{
		Name:      "to-txt",
		Usage:     "Create a DNS TXT records for a discovery tree",
		ArgsUsage: "<tree-directory> <output-file>",
		Action:    dnsToTXT,
		Flags:     []cli.Flag{dnsKeyfileFlag, dnsDomainFlag},
	}
)

var (
	dnsTimeoutFlag = cli.DurationFlag{
		Name:  "timeout",
		Usage: "Timeout for DNS lookups",
	}
	dnsKeyfileFlag = cli.StringFlag{
		Name:  "keyfile",
		Usage: "Key file for signing the tree.",
	}
	dnsDomainFlag = cli.StringFlag{
		Name:  "domain",
		Usage: "Domain name of the tree.",
		Value: "localhost",
	}
)

// dnsSync performs dnsSyncCommand.
func dnsSync(ctx *cli.Context) error {
	var (
		c      = dnsClient(ctx)
		url    = ctx.Args().Get(0)
		outdir = ctx.Args().Get(1)
	)
	domain, _, err := dnsdisc.ParseURL(url)
	if err != nil {
		return err
	}
	if outdir == "" {
		outdir = domain
	}

	t, err := c.SyncTree(url)
	if err != nil {
		return err
	}
	nodes, err := t.Nodes(enode.ValidSchemes)
	if err != nil {
		return err
	}
	meta := dnsMetaJSON{URL: url, Seq: t.Seq(), Links: t.Links()}
	def := &dnsDefinition{Meta: meta, Nodes: nodes}
	writeTreeDefinition(outdir, def)
	return nil
}

// dnsToTXT peforms dnsTXTCommand.
func dnsToTXT(ctx *cli.Context) error {
	var (
		defdir = ctx.Args().Get(0)
		output = ctx.Args().Get(1)
		domain = ctx.String(dnsDomainFlag.Name)
	)
	if ctx.NArg() < 1 {
		return fmt.Errorf("need tree definition directory as argument")
	}
	if output == "" {
		output = "-" // default to stdout
	}

	def := loadTreeDefinition(defdir)
	key := loadSigningKey(ctx)
	t, err := dnsdisc.MakeTree(def.Nodes, def.Meta.Links)
	if err != nil {
		return err
	}
	if _, err := t.Sign(key, def.Meta.Seq, domain); err != nil {
		return fmt.Errorf("Can't sign: %v", err)
	}
	writeTXTJSON(output, t.ToTXT(domain))
	return nil
}

// loadSigningKey loads a private key in Ethereum keystore format.
func loadSigningKey(ctx *cli.Context) *ecdsa.PrivateKey {
	file := ctx.String(dnsKeyfileFlag.Name)
	if file == "" {
		exit(fmt.Errorf("Please specify a key file using the -%s option.", dnsKeyfileFlag.Name))
	}
	keyjson, err := ioutil.ReadFile(file)
	if err != nil {
		exit(fmt.Errorf("Failed to read the keyfile at '%s': %v", file, err))
	}
	password, _ := console.Stdin.PromptPassword("Please enter the password for '%s': ")
	key, err := keystore.DecryptKey(keyjson, password)
	if err != nil {
		exit(fmt.Errorf("Error decrypting key: %v", err))
	}
	return key.PrivateKey
}

// dnsClient configures the DNS discovery client from command line flags.
func dnsClient(ctx *cli.Context) *dnsdisc.Client {
	var cfg dnsdisc.Config
	if commandHasFlag(ctx, dnsTimeoutFlag) {
		cfg.Timeout = ctx.Duration(dnsTimeoutFlag.Name)
	}
	c, _ := dnsdisc.NewClient(cfg) // cannot fail because no URLs given
	return c
}

// There are two file formats for DNS node trees on disk:
//
// The 'TXT' format is a single JSON file containing DNS TXT records
// as a JSON object where the keys are names and the values are objects
// containing the value of the record.
//
// The 'definition' format is a directory containing two files:
//
//      enrtree-info.json    -- contains sequence number & links to other trees
//      nodes.json           -- contains the nodes as a JSON array.
//
// This format exists because it's convenient to edit. nodes.json can be generated
// in multiple ways: it may be written by a DHT crawler or compiled by a human.

type dnsTXTJSON map[string]dnsTXT

type dnsTXT struct {
	Value string `json:"value"`
}

type dnsDefinition struct {
	Meta  dnsMetaJSON
	Nodes []*enode.Node
}

type dnsMetaJSON struct {
	URL   string   `json:"url,omitempty"`
	Seq   uint     `json:"seq"`
	Links []string `json:"links"`
}

// loadTreeDefinition loads a directory in 'definition' format.
func loadTreeDefinition(directory string) *dnsDefinition {
	metaFile, nodesFile := treeDefinitionFiles(directory)
	nodes := loadNodesJSON(nodesFile)
	var def dnsDefinition
	if err := common.LoadJSON(metaFile, &def.Meta); err != nil {
		exit(err)
	}
	// Check link syntax.
	for _, link := range def.Meta.Links {
		if _, _, err := dnsdisc.ParseURL(link); err != nil {
			exit(fmt.Errorf("invalid link %q: %v", link, err))
		}
	}
	// Check/convert nodes.
	def.Nodes = make([]*enode.Node, len(nodes))
	for i, dn := range nodes {
		if dn.ID != dn.Record.ID() {
			exit(fmt.Errorf("invalid node %v: 'id' does not match ID %v from record", dn.ID, dn.Record.ID()))
		}
		def.Nodes[i] = dn.Record
	}
	return &def
}

// writeTreeDefinition writes a DNS node tree definition to the given directory.
func writeTreeDefinition(directory string, def *dnsDefinition) {
	metaJSON, err := json.MarshalIndent(&def.Meta, "", "  ")
	if err != nil {
		exit(err)
	}
	// Convert nodes.
	nodes := make([]nodeJSON, len(def.Nodes))
	for i, n := range def.Nodes {
		nodes[i] = nodeJSON{ID: n.ID(), Seq: n.Seq(), Record: n}
	}
	// Write.
	metaFile, nodesFile := treeDefinitionFiles(directory)
	writeNodesJSON(nodesFile, nodes)
	if err := ioutil.WriteFile(metaFile, metaJSON, 0644); err != nil {
		exit(err)
	}
}

func treeDefinitionFiles(directory string) (string, string) {
	meta := filepath.Join(directory, "enrtree-info.json")
	nodes := filepath.Join(directory, "nodes.json")
	return meta, nodes
}

// loadTXTJSON loads TXT records in JSON format.
func loadTXTJSON(file string) []dnsdisc.TXT {
	var txt dnsTXTJSON
	if err := common.LoadJSON(file, &txt); err != nil {
		exit(err)
	}
	var result []dnsdisc.TXT
	for name, record := range txt {
		result = append(result, dnsdisc.TXT{Name: name, Content: record.Value})
	}
	return result
}

// writeTXTJSON writes TXT records in JSON format.
func writeTXTJSON(file string, records []dnsdisc.TXT) {
	txt := make(dnsTXTJSON)
	for _, r := range records {
		txt[r.Name] = dnsTXT{Value: r.Content}
	}
	txtJSON, err := json.MarshalIndent(txt, "", "  ")
	if err != nil {
		exit(err)
	}
	if file == "-" {
		os.Stdout.Write(txtJSON)
	}
	if err := ioutil.WriteFile(file, txtJSON, 0644); err != nil {
		exit(err)
	}
}
