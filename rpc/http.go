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
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/rs/cors"
)

const (
	maxHTTPRequestContentLength = 1024 * 128
)

type httpClient struct {
	http.Client
	endpoint string
	closed   chan struct{}
}

// httpClient implements clientCodec, but is treated specially by Client.
func (*httpClient) Send(msg interface{}) error    { panic("Send called") }
func (hc *httpClient) Recv(msg interface{}) error { <-hc.closed; return nil }
func (hc *httpClient) Close()                     { close(hc.closed) }

// NewHTTPClient create a new RPC clients that connection to an RPC server over HTTP.
func NewHTTPClient(endpoint string) (*Client, error) {
	if _, err := url.Parse(endpoint); err != nil {
		return nil, err
	}
	hc := &httpClient{endpoint: endpoint, closed: make(chan struct{})}
	return newClient(hc), nil
}

func (c *Client) sendHTTP(op *requestOp, msg interface{}) error {
	hc := c.conn.(*httpClient)
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	resp, err := hc.Post(hc.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var respmsg jsonrpcMessage
	if err := json.NewDecoder(resp.Body).Decode(&respmsg); err != nil {
		return err
	}
	op.resp <- &respmsg
	return nil
}

func (c *Client) sendBatchHTTP(op *requestOp, msgs []*jsonrpcMessage) error {
	hc := c.conn.(*httpClient)
	body, err := json.Marshal(msgs)
	if err != nil {
		return err
	}
	resp, err := hc.Post(hc.endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var respmsgs []jsonrpcMessage
	if err := json.NewDecoder(resp.Body).Decode(&respmsgs); err != nil {
		return err
	}
	for _, respmsg := range respmsgs {
		op.resp <- &respmsg
	}
	return nil
}

// httpReadWriteNopCloser wraps a io.Reader and io.Writer with a NOP Close method.
type httpReadWriteNopCloser struct {
	io.Reader
	io.Writer
}

// Close does nothing and returns always nil
func (t *httpReadWriteNopCloser) Close() error {
	return nil
}

// newJSONHTTPHandler creates a HTTP handler that will parse incoming JSON requests,
// send the request to the given API provider and sends the response back to the caller.
func newJSONHTTPHandler(srv *Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > maxHTTPRequestContentLength {
			http.Error(w,
				fmt.Sprintf("content length too large (%d>%d)", r.ContentLength, maxHTTPRequestContentLength),
				http.StatusRequestEntityTooLarge)
			return
		}

		w.Header().Set("content-type", "application/json")

		// create a codec that reads direct from the request body until
		// EOF and writes the response to w and order the server to process
		// a single request.
		codec := NewJSONCodec(&httpReadWriteNopCloser{r.Body, w})
		defer codec.Close()
		srv.ServeSingleRequest(codec, OptionMethodInvocation)
	}
}

// NewHTTPServer creates a new HTTP RPC server around an API provider.
func NewHTTPServer(corsString string, srv *Server) *http.Server {
	var allowedOrigins []string
	for _, domain := range strings.Split(corsString, ",") {
		allowedOrigins = append(allowedOrigins, strings.TrimSpace(domain))
	}

	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{"POST", "GET"},
	})

	handler := c.Handler(newJSONHTTPHandler(srv))

	return &http.Server{
		Handler: handler,
	}
}
