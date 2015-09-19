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
	"encoding/hex"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

func TestChunkedReader(t *testing.T) {
	checkseq := func(r io.Reader, size uint32) error {
		content := make([]byte, size)
		if _, err := io.ReadFull(r, content); err != nil {
			return err
		}
		for i, b := range content {
			if b != byte(i) {
				return fmt.Errorf("mismatch at index %d: have %d, want %d", i, b, byte(i))
			}
		}
		return nil
	}
	feed := func(cr *chunkedReader, n uint32) {
		for sent := uint32(0); sent < n; {
			chunk := make([]byte, rand.Uint32()%staticFrameSize+1)
			chunklen := uint32(len(chunk))
			if chunklen > n-sent {
				chunklen = n - sent
				chunk = chunk[:chunklen]
			}
			for i := range chunk {
				chunk[i] = byte(uint32(i) + sent)
			}
			sent += chunklen
			if end, _ := cr.feed(bytes.NewBuffer(chunk)); end && sent != n {
				t.Errorf("feed returned end=true with %d bytes of input", sent)
				return
			}
			// pause sometimes to allow the reader to consume.
			if chunklen < 1000 {
				time.Sleep(1 * time.Millisecond)
			}
		}
	}

	for size := uint32(1); size < 2<<17; size *= 2 {
		cr := newChunkedReader(size)
		go feed(cr, size)
		if err := checkseq(cr, size); err != nil {
			t.Fatalf("size %d read error: %v", size, err)
		}
	}
}

func TestFrameFakeGold(t *testing.T) {
	buf := new(bytes.Buffer)
	hash := fakeHash{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	rw := newFrameRW(buf, secrets{
		AES:        crypto.Sha3(),
		MAC:        crypto.Sha3(),
		IngressMAC: hash,
		EgressMAC:  hash,
	})

	golden := unhex(`
00828ddae471818bb0bfa6b551d1cb42
01010101010101010101010101010101
ba628a4ba590cb43f7848f41c4382885
01010101010101010101010101010101
`)
	body := unhex(`08C401020304`)

	// Check sendFrame. This puts a message into the buffer.
	if err := rw.sendFrame(regularHeader{0, 0}, bytes.NewReader(body), uint32(len(body))); err != nil {
		t.Fatalf("sendFrame error: %v", err)
	}
	written := buf.Bytes()
	if !bytes.Equal(written, golden) {
		t.Fatalf("output mismatch:\n  got:  %x\n  want: %x", written, golden)
	}

	// Check ReadMsg. It reads the message encoded by WriteMsg, which
	// is equivalent to the golden message above.
	hdr, bodybuf, err := rw.readFrame()
	if err != nil {
		t.Fatalf("ReadMsg error: %v", err)
	}
	if (hdr != frameHeader{}) {
		t.Errorf("read header mismatch: got %v, want zero header", hdr)
	}
	if !bytes.Equal(bodybuf.Bytes(), body) {
		t.Errorf("read body mismatch:\ngot  %x\nwant %x", bodybuf.Bytes(), body)
	}
}

type fakeHash []byte

func (fakeHash) Write(p []byte) (int, error) { return len(p), nil }
func (fakeHash) Reset()                      {}
func (fakeHash) BlockSize() int              { return 0 }

func (h fakeHash) Size() int           { return len(h) }
func (h fakeHash) Sum(b []byte) []byte { return append(b, h...) }

func unhex(str string) []byte {
	b, err := hex.DecodeString(strings.Replace(str, "\n", "", -1))
	if err != nil {
		panic(fmt.Sprintf("invalid hex string: %q", str))
	}
	return b
}
