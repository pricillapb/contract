// Copyright 2015 The go-ethereum Authors
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

package rlpx

import (
	"bytes"
	"crypto/ecdsa"
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/ecies"
)

func init() {
	spew.Config.Indent = "\t"
}

func TestSharedSecret(t *testing.T) {
	prv0, _ := crypto.GenerateKey()
	pub0 := &prv0.PublicKey
	prv1, _ := crypto.GenerateKey()
	pub1 := &prv1.PublicKey
	ss0, err := ecies.ImportECDSA(prv0).GenerateShared(ecies.ImportECDSAPublic(pub1), sskLen, sskLen)
	if err != nil {
		return
	}
	ss1, err := ecies.ImportECDSA(prv1).GenerateShared(ecies.ImportECDSAPublic(pub0), sskLen, sskLen)
	if err != nil {
		return
	}
	if !bytes.Equal(ss0, ss1) {
		t.Errorf("secret mismatch")
	}
}

func TestHandshake(t *testing.T) {
	for i := 0; i < 10; i++ {
		start := time.Now()
		if err := doTestHandshake(); err != nil {
			t.Fatalf("i=%d %v", i, err)
		}
		t.Logf("%d %v\n", i+1, time.Since(start))
	}
}

func doTestHandshake() error {
	var (
		prv0, _  = crypto.GenerateKey()
		prv1, _  = crypto.GenerateKey()
		fd0, fd1 = net.Pipe()
		c0       = Server(fd0, &Config{Key: prv0})
		c1       = Client(fd1, &prv0.PublicKey, &Config{Key: prv1})
	)
	type result struct {
		side string
		err  error
	}
	output := make(chan result)
	shake := func(side string, conn *Conn, rkey *ecdsa.PublicKey) {
		r := result{side: side}
		defer func() { output <- r }()
		if r.err = conn.Handshake(); r.err != nil {
			return
		}
		if !reflect.DeepEqual(conn.RemoteID(), rkey) {
			r.err = fmt.Errorf("remote ID mismatch: got %v, want: %v", conn.RemoteID(), rkey)
		}
	}
	go shake("initiator", c0, &prv1.PublicKey)
	go shake("recipient", c1, &prv0.PublicKey)

	// wait for results from both sides
	r1, r2 := <-output, <-output
	if r1.err != nil {
		return fmt.Errorf("%s side error: %v", r1.side, r1.err)
	}
	if r2.err != nil {
		return fmt.Errorf("%s side error: %v", r2.side, r2.err)
	}

	// compare derived secrets
	if !reflect.DeepEqual(c0.rw.egressMac, c1.rw.ingressMac) {
		return fmt.Errorf("egress mac mismatch:\n c0.rw: %#v\n c1.rw: %#v", c0.rw.egressMac, c1.rw.ingressMac)
	}
	if !reflect.DeepEqual(c0.rw.ingressMac, c1.rw.egressMac) {
		return fmt.Errorf("ingress mac mismatch:\n c0.rw: %#v\n c1.rw: %#v", c0.rw.ingressMac, c1.rw.egressMac)
	}
	if !reflect.DeepEqual(c0.rw.enc, c1.rw.enc) {
		return fmt.Errorf("enc cipher mismatch:\n c0.rw: %#v\n c1.rw: %#v", c0.rw.enc, c1.rw.enc)
	}
	if !reflect.DeepEqual(c0.rw.dec, c1.rw.dec) {
		return fmt.Errorf("dec cipher mismatch:\n c0.rw: %#v\n c1.rw: %#v", c0.rw.dec, c1.rw.dec)
	}
	return nil
}

func TestInitiatorHandshakeTV(t *testing.T) {
	for i, ht := range handshakeTV {
		p1, p2 := net.Pipe()
		remotePub := &ht.recipientConfig.Key.PublicKey
		conn := Client(p2, remotePub, ht.initiatorConfig)
		conn.handshakeRand = fakeRandSource{key: ht.initiatorEphemeralKey, nonce: ht.initiatorNonce}
		go run(t, rig{
			"auth packet": func() error {
				buf := make([]byte, len(ht.auth))
				if _, err := io.ReadFull(p1, buf); err != nil {
					return err
				}
				return nil
			},
			"authResp packet": func() error {
				_, err := p1.Write(ht.authResp)
				return err
			},
		})
		ingress, egress, err := conn.initiatorHandshake()
		if err != nil {
			t.Errorf("handshake error: %v", err)
		}

		ingress.mac, egress.mac = nil, nil // remove mac hashes so they compare
		if !reflect.DeepEqual(ingress, ht.initiatorIngressSecrets) {
			t.Errorf("ingress secrets mismatch:\ngot %swant %s", spew.Sdump(ingress), spew.Sdump(ht.initiatorIngressSecrets))
		}
		if !reflect.DeepEqual(egress, ht.initiatorEgressSecrets) {
			t.Errorf("egress secrets mismatch:\ngot %swant %s", spew.Sdump(egress), spew.Sdump(ht.initiatorEgressSecrets))
		}
		if t.Failed() {
			t.Fatalf("failed test case %d:\n%s", i, spew.Sdump(ht))
		}
	}
}

func TestRecipientHandshakeTV(t *testing.T) {
	for i, ht := range handshakeTV {
		p1, p2 := net.Pipe()
		conn := Server(p2, ht.recipientConfig)
		conn.handshakeRand = fakeRandSource{key: ht.recipientEphemeralKey, nonce: ht.recipientNonce}
		go run(t, rig{
			"authResp packet": func() error {
				buf := make([]byte, len(ht.authResp))
				if _, err := io.ReadFull(p1, buf); err != nil {
					return err
				}
				return nil
			},
			"auth packet": func() error {
				_, err := p1.Write(ht.auth)
				return err
			},
		})
		remoteID, ingress, egress, err := conn.recipientHandshake()
		if err != nil {
			t.Errorf("handshake error: %v", err)
		}

		if !reflect.DeepEqual(remoteID, &ht.initiatorConfig.Key.PublicKey) {
			t.Errorf("remoteID mismatch:\ngot  %x\nwant %x", crypto.FromECDSAPub(remoteID), crypto.FromECDSAPub(&ht.initiatorConfig.Key.PublicKey))
		}
		ingress.mac, egress.mac = nil, nil // remove mac hashes so they compare
		if !reflect.DeepEqual(ingress, ht.initiatorEgressSecrets) {
			t.Errorf("ingress secrets mismatch:\ngot %swant %s", spew.Sdump(ingress), spew.Sdump(ht.initiatorEgressSecrets))
		}
		if !reflect.DeepEqual(egress, ht.initiatorIngressSecrets) {
			t.Errorf("egress secrets mismatch:\ngot %swant %s", spew.Sdump(egress), spew.Sdump(ht.initiatorIngressSecrets))
		}
		if t.Failed() {
			t.Fatalf("failed test case %d:\n%s", i, spew.Sdump(ht))
		}
	}
}

type fakeRandSource struct {
	key   *ecdsa.PrivateKey
	nonce []byte
}

func (ht fakeRandSource) generateNonce(b []byte) error {
	if len(b) != len(ht.nonce) {
		panic(fmt.Sprintf("requested nonce of size %d, have %d", len(b), len(ht.nonce)))
	}
	copy(b, ht.nonce)
	return nil
}

func (ht fakeRandSource) generateKey() (*ecies.PrivateKey, error) {
	return ecies.ImportECDSA(ht.key), nil
}
