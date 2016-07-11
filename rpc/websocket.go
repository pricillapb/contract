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

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"golang.org/x/net/websocket"
	"gopkg.in/fatih/set.v0"
)

// wsHandshakeValidator returns a handler that verifies the origin during the
// websocket upgrade process. When a '*' is specified as an allowed origins all
// connections are accepted.
func wsHandshakeValidator(allowedOrigins []string) func(*websocket.Config, *http.Request) error {
	origins := set.New()
	allowAllOrigins := false

	for _, origin := range allowedOrigins {
		if origin == "*" {
			allowAllOrigins = true
		}
		if origin != "" {
			origins.Add(origin)
		}
	}

	// allow localhost if no allowedOrigins are specified
	if len(origins.List()) == 0 {
		origins.Add("http://localhost")
		if hostname, err := os.Hostname(); err == nil {
			origins.Add("http://" + hostname)
		}
	}

	glog.V(logger.Debug).Infof("Allowed origin(s) for WS RPC interface %v\n", origins.List())

	f := func(cfg *websocket.Config, req *http.Request) error {
		origin := req.Header.Get("Origin")
		if allowAllOrigins || origins.Has(origin) {
			return nil
		}
		glog.V(logger.Debug).Infof("origin '%s' not allowed on WS-RPC interface\n", origin)
		return fmt.Errorf("origin %s not allowed", origin)
	}

	return f
}

// NewWSServer creates a new websocket RPC server around an API provider.
func NewWSServer(allowedOrigins string, handler *Server) *http.Server {
	return &http.Server{
		Handler: websocket.Server{
			Handshake: wsHandshakeValidator(strings.Split(allowedOrigins, ",")),
			Handler: func(conn *websocket.Conn) {
				handler.ServeCodec(NewJSONCodec(conn), OptionMethodInvocation|OptionSubscriptions)
			},
		},
	}
}

func ListenWS(s *Server, addr, allowedOrigins string) (net.Listener, error) {
	hs := NewWSServer(allowedOrigins, s)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, err
	}
	go hs.Serve(listener)
	return listener, nil
}

// DialWS creates a new RPC client that communicates with a JSON-RPC server
// that is listening on the given endpoint.
func DialWS(endpoint, origin string) (*Client, error) {
	return newClient(func() (net.Conn, error) {
		if origin == "" {
			var err error
			if origin, err = os.Hostname(); err != nil {
				return nil, err
			}
			if strings.HasPrefix(endpoint, "wss") {
				origin = "https://" + origin
			} else {
				origin = "http://" + origin
			}
		}
		return websocket.Dial(endpoint, "", origin)
	})
}
