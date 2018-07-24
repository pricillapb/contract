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

package enr

import (
	"crypto/ecdsa"
	"fmt"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/sha3"
	"github.com/ethereum/go-ethereum/rlp"
)

// An IdentityScheme is capable of verifying record signatures and
// deriving node addresses.
type IdentityScheme interface {
	Verify(r *Record, sig []byte) error
	NodeAddr(r *Record) []byte
}

// SchemeMap is a registry of named identity schemes.
type SchemeMap map[string]IdentityScheme

func (m SchemeMap) Verify(r *Record, sig []byte) error {
	s, _ := m[r.IdentityScheme()]
	if s == nil {
		return errInvalidSig
	}
	return s.Verify(r, sig)
}

func (m SchemeMap) NodeAddr(r *Record) []byte {
	s, _ := m[r.IdentityScheme()]
	if s == nil {
		return nil
	}
	return s.NodeAddr(r)
}

// v4ID is the "v4" identity scheme.
type V4ID struct{}

// SignV4 signs a record using the v4 scheme.
func SignV4(r *Record, privkey *ecdsa.PrivateKey) error {
	// Copy r to avoid modifying it if signing fails.
	cpy := *r
	cpy.Set(ID("v4"))
	cpy.Set(Secp256k1(privkey.PublicKey))

	h := sha3.NewKeccak256()
	rlp.Encode(h, cpy.AppendElements(nil))
	sig, err := crypto.Sign(h.Sum(nil), privkey)
	if err != nil {
		return err
	}
	sig = sig[:len(sig)-1] // remove v
	if err = cpy.SetSig(V4ID{}, sig); err == nil {
		*r = cpy
	}
	return err
}

// s256raw is an unparsed secp256k1 public key entry.
type s256raw []byte

func (s256raw) ENRKey() string { return "secp256k1" }

func (V4ID) Verify(r *Record, sig []byte) error {
	var entry s256raw
	if err := r.Load(&entry); err != nil {
		return err
	} else if len(entry) != 33 {
		return fmt.Errorf("invalid public key")
	}

	h := sha3.NewKeccak256()
	rlp.Encode(h, r.AppendElements(nil))
	if !crypto.VerifySignature(entry, h.Sum(nil), sig) {
		return errInvalidSig
	}
	return nil
}

func (V4ID) NodeAddr(r *Record) []byte {
	var pubkey Secp256k1
	err := r.Load(&pubkey)
	if err != nil {
		return nil
	}
	buf := make([]byte, 64)
	math.ReadBits(pubkey.X, buf[:32])
	math.ReadBits(pubkey.Y, buf[32:])
	return crypto.Keccak256(buf)
}
