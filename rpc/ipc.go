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

package rpc

import "net"

// CreateIPCListener creates an listener, on Unix platforms this is a unix socket, on Windows this is a named pipe
func CreateIPCListener(endpoint string) (net.Listener, error) {
	return ipcListen(endpoint)
}

// DialIPC create a new IPC client that will connect on the given endpoint. Messages are JSON encoded and encoded.
// On Unix it assumes the endpoint is the full path to a unix socket, and Windows the endpoint is an identifier for a
// named pipe.
func DialIPC(endpoint string) (*Client, error) {
	return newClient(func() (net.Conn, error) {
		return newIPCConnection(endpoint)
	})
}
