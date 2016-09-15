// Copyright 2014 The go-ethereum Authors
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

// Package common contains various helper functions.
package common

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

func ToHex(b []byte) string {
	hex := Bytes2Hex(b)
	// Prefer output of "0x0" instead of "0x"
	if len(hex) == 0 {
		hex = "0"
	}
	return "0x" + hex
}

func FromHex(s string) []byte {
	if len(s) > 1 {
		if s[0:2] == "0x" {
			s = s[2:]
		}
		if len(s)%2 == 1 {
			s = "0" + s
		}
		return Hex2Bytes(s)
	}
	return nil
}

// Number to bytes
//
// Returns the number in bytes with the specified base
func NumberToBytes(num interface{}, bits int) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, num)
	if err != nil {
		fmt.Println("NumberToBytes failed:", err)
	}

	return buf.Bytes()[buf.Len()-(bits/8):]
}

// Bytes to number
//
// Attempts to cast a byte slice to a unsigned integer
func BytesToNumber(b []byte) uint64 {
	var number uint64

	// Make sure the buffer is 64bits
	data := make([]byte, 8)
	data = append(data[:len(b)], b...)

	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.BigEndian, &number)
	if err != nil {
		fmt.Println("BytesToNumber failed:", err)
	}

	return number
}

// Read variable int
//
// Read a variable length number in big endian byte order
func ReadVarInt(buff []byte) (ret uint64) {
	switch l := len(buff); {
	case l > 4:
		d := LeftPadBytes(buff, 8)
		binary.Read(bytes.NewReader(d), binary.BigEndian, &ret)
	case l > 2:
		var num uint32
		d := LeftPadBytes(buff, 4)
		binary.Read(bytes.NewReader(d), binary.BigEndian, &num)
		ret = uint64(num)
	case l > 1:
		var num uint16
		d := LeftPadBytes(buff, 2)
		binary.Read(bytes.NewReader(d), binary.BigEndian, &num)
		ret = uint64(num)
	default:
		var num uint8
		binary.Read(bytes.NewReader(buff), binary.BigEndian, &num)
		ret = uint64(num)
	}

	return
}

// Copy bytes
//
// Returns an exact copy of the provided bytes
func CopyBytes(b []byte) (copiedBytes []byte) {
	copiedBytes = make([]byte, len(b))
	copy(copiedBytes, b)

	return
}

func HasHexPrefix(str string) bool {
	l := len(str)
	return l >= 2 && str[0:2] == "0x"
}

func IsHex(str string) bool {
	l := len(str)
	return l >= 4 && l%2 == 0 && str[0:2] == "0x"
}

func Bytes2Hex(d []byte) string {
	return hex.EncodeToString(d)
}

func Hex2Bytes(str string) []byte {
	h, _ := hex.DecodeString(str)

	return h
}

func RightPadBytes(slice []byte, l int) []byte {
	if l < len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded[0:len(slice)], slice)

	return padded
}

func LeftPadBytes(slice []byte, l int) []byte {
	if l < len(slice) {
		return slice
	}

	padded := make([]byte, l)
	copy(padded[l-len(slice):], slice)

	return padded
}

func LeftPadString(str string, l int) string {
	if l < len(str) {
		return str
	}

	zeros := Bytes2Hex(make([]byte, (l-len(str))/2))

	return zeros + str

}

func RightPadString(str string, l int) string {
	if l < len(str) {
		return str
	}

	zeros := Bytes2Hex(make([]byte, (l-len(str))/2))

	return str + zeros

}
