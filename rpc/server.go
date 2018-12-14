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
	"context"
	"io"
	"sync"
	"sync/atomic"

	mapset "github.com/deckarep/golang-set"
	"github.com/ethereum/go-ethereum/log"
)

const MetadataApi = "rpc"

// CodecOption specifies which type of messages this codec supports
type CodecOption int

const (
	// OptionMethodInvocation is an indication that the codec supports RPC method calls
	OptionMethodInvocation CodecOption = 1 << iota

	// OptionSubscriptions is an indication that the codec suports RPC notifications
	OptionSubscriptions = 1 << iota // support pub sub
)

// Server represents a RPC server
type Server struct {
	services serviceRegistry

	run      int32
	codecsMu sync.Mutex
	codecs   mapset.Set
}

// NewServer creates a new server instance with no registered handlers.
func NewServer() *Server {
	server := &Server{codecs: mapset.NewSet(), run: 1}
	// Register the default service providing meta information about the RPC service such
	// as the services and methods it offers.
	rpcService := &RPCService{server}
	server.RegisterName(MetadataApi, rpcService)
	return server
}

// RegisterName creates a service for the given rcvr type under the given name. When no
// methods on the given rcvr match the criteria to be either a RPC method or a subscription
// an error is returned. Otherwise a new service is created and added to the service
// collection this server provides.
func (s *Server) RegisterName(name string, rcvr interface{}) error {
	return s.services.registerName(name, rcvr)
}

// serveRequest reads requests from the codec, calls the RPC callback and writes the
// response to the given codec.
//
// If singleShot is true it will process a single request, otherwise it will handle
// requests until the codec returns an error when reading a request. Requests are executed
// concurrently when singleShot is false.
func (s *Server) serveRequest(ctx context.Context, codec ServerCodec, singleShot bool, options CodecOption) error {
	var pend sync.WaitGroup
	defer pend.Wait()

	// Add the codec and remove it on shutdown.
	s.codecsMu.Lock()
	s.codecs.Add(codec)
	s.codecsMu.Unlock()
	defer func() {
		s.codecsMu.Lock()
		s.codecs.Remove(codec)
		s.codecsMu.Unlock()
	}()

	// Serve requests until shutdown.
	handler := newHandler(&s.services)
	for atomic.LoadInt32(&s.run) == 1 {
		reqs, batch, err := codec.Read()
		if err != nil {
			if err != io.EOF {
				log.Debug("RPC connection error", "err", err)
			}
			break
		}
		// Respond with error on shutdown.
		if atomic.LoadInt32(&s.run) != 1 {
			err = &shutdownError{}
			if batch {
				resps := make([]interface{}, len(reqs))
				for i, r := range reqs {
					resps[i] = r.errorResponse(err)
				}
				codec.Write(resps)
			} else {
				codec.Write(reqs[0].errorResponse(err))
			}
			break
		}

		// Serve the request.
		pend.Add(1)
		go func() {
			defer pend.Done()
			// This is the request-scoped context.
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()
			// If the codec supports notification include a notifier that callbacks can use
			// to send notification to clients. It is tied to the request. If the
			// connection is closed the notifier will stop and cancels all active
			// subscriptions.
			if options&OptionSubscriptions == OptionSubscriptions {
				sn := newServerNotifier(codec)
				ctx = context.WithValue(ctx, serverNotifierKey{}, sn)
				defer sn.activate()
			}
			var resp interface{}
			if batch {
				resp = handler.handleBatch(ctx, reqs)
			} else {
				resp = handler.handleMsg(ctx, reqs[0])
			}
			codec.Write(resp)
		}()

		if singleShot {
			break
		}
	}
	return nil
}

// ServeCodec reads incoming requests from codec, calls the appropriate callback and writes
// the response back using the given codec. It will block until the codec is closed or the
// server is stopped. In either case the codec is closed.
func (s *Server) ServeCodec(codec ServerCodec, options CodecOption) {
	defer codec.Close()
	s.serveRequest(context.Background(), codec, false, options)
}

// ServeSingleRequest reads and processes a single RPC request from the given codec. It
// will not close the codec unless a non-recoverable error has occurred. Note, this method
// will return after a single request has been processed!
func (s *Server) ServeSingleRequest(ctx context.Context, codec ServerCodec, options CodecOption) {
	s.serveRequest(ctx, codec, true, options)
}

// Stop stops reading new requests, waits for stopPendingRequestTimeout to allow pending
// requests to finish, then closes all codecs which will cancel pending requests and
// subscriptions.
func (s *Server) Stop() {
	if atomic.CompareAndSwapInt32(&s.run, 1, 0) {
		log.Debug("RPC server shutting down")
		s.codecsMu.Lock()
		defer s.codecsMu.Unlock()
		s.codecs.Each(func(c interface{}) bool {
			c.(ServerCodec).Close()
			return true
		})
	}
}

// RPCService gives meta information about the server.
// e.g. gives information about the loaded modules.
type RPCService struct {
	server *Server
}

// Modules returns the list of RPC services with their version number
func (s *RPCService) Modules() map[string]string {
	s.server.services.mu.Lock()
	defer s.server.services.mu.Unlock()

	modules := make(map[string]string)
	for name := range s.server.services.services {
		modules[name] = "1.0"
	}
	return modules
}
