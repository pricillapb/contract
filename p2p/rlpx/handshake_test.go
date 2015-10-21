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
	for i, ht := range initiatorHandshakeTV {
		p1, p2 := net.Pipe()
		remotePub := crypto.ToECDSAPub(ht.remoteID)
		localPrivKey := crypto.ToECDSA(ht.privKey)
		conn := Client(p2, remotePub, &Config{Key: localPrivKey})
		conn.handshakeRand = ht
		go run(t, rig{
			"auth packet": func() error {
				buf := make([]byte, len(ht.auth))
				if _, err := io.ReadFull(p1, buf); err != nil {
					return err
				}
				if !bytes.Equal(buf, ht.auth) {
					return fmt.Errorf("mismatch:\ngot %swant %s", spew.Sdump(buf), spew.Sdump(ht.auth))
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
		if !reflect.DeepEqual(ingress, ht.wantIngress) {
			t.Errorf("ingress secrets mismatch:\ngot %swant %s", spew.Sdump(ingress), spew.Sdump(ht.wantIngress))
		}
		if !reflect.DeepEqual(egress, ht.wantEgress) {
			t.Errorf("egress secrets mismatch:\ngot %swant %s", spew.Sdump(egress), spew.Sdump(ht.wantEgress))
		}
		if t.Failed() {
			t.Fatalf("failed test case %d:\n%s", i, spew.Sdump(ht))
		}
	}
}

// func init() {
// 	key, _ := crypto.GenerateKey()
// 	fmt.Printf("a key: %x\n", crypto.FromECDSA(key))
// }

var initiatorHandshakeTV = []handshakeTest{
	// old V4 test vectors from https://gist.github.com/fjl/3a78780d17c755d22df2
	{
		privKey:          unhex("5e173f6ac3c669587538e7727cf19b782a4f2fda07c1eaa662c593e5e85e3051"),
		remoteID:         unhex("0444cfe44ddb7ccb045a470bd486cfb84ce363181e4ee65810fee243bfef4cc68bf6c90c5669bbac06dc29a77515cc5e3302edaea383ce1cf0a805658f1641a855"),
		randEphemeralKey: unhex("19c2185f4f40634926ebed3af09070ca9e029f2edd5fae6253074896205f5f6c"),
		randNonce:        unhex("cd26fecb93657d1cd9e9eaf4f8be720b56dd1d39f190c4e1c6b7ec66f077bb11"),

		auth: unhex(`
			04a0274c5951e32132e7f088c9bdfdc76c9d91f0dc6078e848f8e3361193dbdc
			43b94351ea3d89e4ff33ddcefbc80070498824857f499656c4f79bbd97b6c51a
			514251d69fd1785ef8764bd1d262a883f780964cce6a14ff206daf1206aa073a
			2d35ce2697ebf3514225bef186631b2fd2316a4b7bcdefec8d75a1025ba2c540
			4a34e7795e1dd4bc01c6113ece07b0df13b69d3ba654a36e35e69ff9d482d88d
			2f0228e7d96fe11dccbb465a1831c7d4ad3a026924b182fc2bdfe016a6944312
			021da5cc459713b13b86a686cf34d6fe6615020e4acf26bf0d5b7579ba813e77
			23eb95b3cef9942f01a58bd61baee7c9bdd438956b426a4ffe238e61746a8c93
			d5e10680617c82e48d706ac4953f5e1c4c4f7d013c87d34a06626f498f34576d
			c017fdd3d581e83cfd26cf125b6d2bda1f1d56
		`),
		authResp: unhex(`
			049934a7b2d7f9af8fd9db941d9da281ac9381b5740e1f64f7092f3588d4f87f
			5ce55191a6653e5e80c1c5dd538169aa123e70dc6ffc5af1827e546c0e958e42
			dad355bcc1fcb9cdf2cf47ff524d2ad98cbf275e661bf4cf00960e74b5956b79
			9771334f426df007350b46049adb21a6e78ab1408d5e6ccde6fb5e69f0f4c92b
			b9c725c02f99fa72b9cdc8dd53cff089e0e73317f61cc5abf6152513cb7d833f
			09d2851603919bf0fbe44d79a09245c6e8338eb502083dc84b846f2fee1cc310
			d2cc8b1b9334728f97220bb799376233e113
		`),

		wantIngress: secrets{
			encKey: unhex("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  unhex("00000000000000000000000000000000"),
			macKey: unhex("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
		wantEgress: secrets{
			encKey: unhex("c0458fa97a5230830e05f4f20b7c755c1d4e54b1ce5cf43260bb191eef4e418d"),
			encIV:  unhex("00000000000000000000000000000000"),
			macKey: unhex("48c938884d5067a1598272fcddaa4b833cd5e7d92e8228c0ecdfabbe68aef7f1"),
		},
	},
}

type handshakeTest struct {
	// inputs
	privKey, remoteID, auth, authResp []byte
	// random bits (injected through handshakeRandSource methods below)
	randNonce, randEphemeralKey []byte
	// outputs
	wantIngress, wantEgress secrets
	remoteIDout             []byte
}

func (ht handshakeTest) generateNonce(b []byte) error {
	if len(b) != len(ht.randNonce) {
		panic(fmt.Sprintf("requested nonce of size %d, have %d", len(b), len(ht.randNonce)))
	}
	copy(b, ht.randNonce)
	return nil
}

func (ht handshakeTest) generateKey() (*ecies.PrivateKey, error) {
	return ecies.ImportECDSA(crypto.ToECDSA(ht.randEphemeralKey)), nil
}
