// Copyright 2016 The go-ethereum Authors
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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
)

var (
	ErrClientQuit = errors.New("client is closed")
	ErrNoResult   = errors.New("no result in JSON-RPC response")
)

const clientSubscriptionBuffer = 100

type clientCodec interface {
	Send(msg interface{}) error
	Recv(msg interface{}) error
	Close()
}

// BatchElem is an element in a batch request.
type BatchElem struct {
	Method string
	Args   []interface{}
	// The result is unmarshaled into this field.
	// Result must be set to a non-nil value of the desired type, otherwise
	// the response will be discarded.
	Result interface{}
	// Error is set if the server returns an error for this request,
	// or if unmarshaling into Result fails.
	// It is not set for I/O errors.
	Error error
}

// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *JSONError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) isNotification() bool {
	return msg.ID == nil && msg.Method != ""
}

func (msg *jsonrpcMessage) isResponse() bool {
	return msg.hasValidID() && msg.Method == "" && len(msg.Params) == 0
}

func (msg *jsonrpcMessage) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *jsonrpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

type Client struct {
	conn      clientCodec
	wg        sync.WaitGroup
	idCounter uint32

	connectFunc func() (net.Conn, error)

	// for dispatch
	didQuit   chan struct{}                  // closed when client quits
	readErr   chan error                     // errors from read
	readResp  chan []*jsonrpcMessage         // valid messages from read
	requestOp chan *requestOp                // for registering response IDs
	sendDone  chan error                     // signals write completion, releases write lock
	respWait  map[string]*requestOp          // active requests
	subs      map[string]*ClientSubscription // active subscriptions
}

type requestOp struct {
	ids []json.RawMessage
	// set for requests, batch requests
	err  error
	resp chan *jsonrpcMessage
	// set for subscriptions
	sub *ClientSubscription
}

func (op *requestOp) wait(ctx context.Context) (*jsonrpcMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-op.resp:
		return resp, op.err
	}
}

// Dial creates a new client for the given URL.
//
// The currently supported URL schemes are "http", "https", "ws" and "wss". If rawurl is a
// file name with no URL scheme, a local socket connection is established using UNIX
// domain sockets on supported platforms and named pipes on Windows. If you want to
// customize the transport, use DialHTTP, DialWS or DialIPC instead.
//
// The client reconnects automatically if the connection is lost.
func Dial(rawurl string) (*Client, error) {
	u, err := url.Parse(rawurl)
	if err != nil {
		return nil, err
	}
	switch u.Scheme {
	case "http", "https":
		return DialHTTP(rawurl)
	case "ws", "wss":
		return DialWS(rawurl)
	case "":
		return DialIPC(rawurl)
	default:
		return nil, fmt.Errorf("no known transport for URL scheme %q", u.Scheme)
	}
}

func newClient(codec clientCodec) *Client {
	c := &Client{
		conn:      codec,
		didQuit:   make(chan struct{}),
		readErr:   make(chan error),
		readResp:  make(chan []*jsonrpcMessage),
		requestOp: make(chan *requestOp),
		sendDone:  make(chan error, 1),
		respWait:  make(map[string]*requestOp),
		subs:      make(map[string]*ClientSubscription),
	}
	c.wg.Add(2)
	go c.read()
	go c.dispatch()
	return c
}

func (c *Client) nextID() json.RawMessage {
	id := atomic.AddUint32(&c.idCounter, 1)
	return []byte(strconv.FormatUint(uint64(id), 10))
}

func (c *Client) SupportedModules() (map[string]string, error) {
	var result map[string]string
	err := c.Call(&result, "rpc_modules")
	return result, err
}

// Close closes the client, aborting any in-flight requests.
func (c *Client) Close() {
	c.conn.Close()
	c.wg.Wait()
}

// Request performs a JSON-RPC call with the given arguments
// and unmarshals into result if no error occurred.
func (c *Client) Call(result interface{}, method string, args ...interface{}) error {
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, resp: make(chan *jsonrpcMessage, 1)}

	if _, ok := c.conn.(*httpClient); ok {
		err = c.sendHTTP(op, msg)
	} else {
		err = c.send(op, msg)
	}
	if err != nil {
		return err
	}

	// dispatch has accepted the request and will close the channel it when it quits.
	switch resp, err := op.wait(context.TODO()); {
	case err != nil:
		return err
	case resp.Error != nil:
		return resp.Error
	case len(resp.Result) == 0:
		return ErrNoResult
	default:
		return json.Unmarshal(resp.Result, &result)
	}
}

// BatchRequest sends all given requests as a single batch
// and waits for the server to return a response for all of them.
//
// In contrast to Request, BatchRequest only returns I/O errors.
// Any error specific to a request is reported through the Error field
// of the corresponding BatchElem.
func (c *Client) BatchCall(b []BatchElem) error {
	msgs := make([]*jsonrpcMessage, len(b))
	op := &requestOp{
		ids:  make([]json.RawMessage, len(b)),
		resp: make(chan *jsonrpcMessage, len(b)),
	}
	for i, elem := range b {
		msg, err := c.newMessage(elem.Method, elem.Args...)
		if err != nil {
			return err
		}
		msgs[i] = msg
		op.ids[i] = msg.ID
	}

	var err error
	if _, ok := c.conn.(*httpClient); ok {
		err = c.sendBatchHTTP(op, msgs)
	} else {
		err = c.send(op, msgs)
	}

	// wait for all responses to come back.
	for n := 0; n < len(b) && err == nil; n++ {
		var resp *jsonrpcMessage
		resp, err = op.wait(context.TODO())
		if err != nil {
			break
		}
		// Find the element corresponding to this response.
		// The element is guaranteed to be present because dispatch
		// only sends valid IDs to our channel.
		var elem *BatchElem
		for i := range msgs {
			if bytes.Equal(msgs[i].ID, resp.ID) {
				elem = &b[i]
				break
			}
		}
		if resp.Error != nil {
			elem.Error = resp.Error
			continue
		}
		if len(resp.Result) == 0 {
			elem.Error = ErrNoResult
			continue
		}
		elem.Error = json.Unmarshal(resp.Result, elem.Result)
	}
	return err
}

// EthSubscribe calls the "eth_subscribe" method with the given arguments, registering a
// subscription. Server notifications for the subscription are sent to the given channel.
// The element type of the channel must match the expected type of content returned by the
// subscription.
//
// The channel is closed when the notification is unsubscribed or an error occurs.
// The error can be retrieved via the Err method of the subscription.
//
// Slow subscribers will block the clients ingress path eventually.
func (c *Client) EthSubscribe(channel interface{}, args ...interface{}) (*ClientSubscription, error) {
	// Check type of channel first.
	chanVal := reflect.ValueOf(channel)
	if chanVal.Kind() != reflect.Chan || chanVal.Type().ChanDir()&reflect.SendDir == 0 {
		panic("first argument to EthSubscribe must be a writable channel")
	}
	if chanVal.IsNil() {
		panic("channel given to EthSubscribe must not be nil")
	}
	// HTTP and notifications don't mix.
	if _, ok := c.conn.(*httpClient); ok {
		return nil, ErrNotificationsUnsupported
	}

	msg, err := c.newMessage(subscribeMethod, args...)
	if err != nil {
		return nil, err
	}
	sub := newClientSubscription(c, chanVal)
	op := &requestOp{ids: []json.RawMessage{msg.ID}, sub: sub}
	if err := c.send(op, msg); err != nil {
		return nil, err
	}
	// The arrival and validity of the response is signaled on sub.quit.
	if err := <-op.sub.quit; err != nil {
		return nil, err
	}
	return op.sub, nil
}

func (c *Client) newMessage(method string, paramsIn ...interface{}) (*jsonrpcMessage, error) {
	params, err := json.Marshal(paramsIn)
	if err != nil {
		return nil, err
	}
	return &jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: method, Params: params}, nil
}

// send registers op with the dispatch loop, then sends msg on the connection.
// if sending fails, op is deregistered.
func (c *Client) send(op *requestOp, msg interface{}) error {
	select {
	case c.requestOp <- op:
		if glog.V(logger.Detail) {
			glog.Info("sending ", msg)
		}
		err := c.conn.Send(msg)
		c.sendDone <- err
		return err
	case <-c.didQuit:
		return ErrClientQuit
	}
}

// dispatch is the main loop of the client.
// It sends read messages to waiting calls to Request and BatchRequest
// and subscription notifications to registered subscriptions.
func (c *Client) dispatch() {
	defer c.wg.Done()
	defer c.conn.Close()
	defer close(c.didQuit)
	defer c.closeRequestOps(ErrClientQuit)

	var (
		lastOp        *requestOp
		requestOpLock = c.requestOp // nil while the send lock is held
		disconnected  = true
	)
	for {
		select {
		// Read path.
		case err := <-c.readErr:
			glog.V(logger.Debug).Infof("<-readErr: %v", err)
			disconnected = true
			c.closeRequestOps(err)
		case batch := <-c.readResp:
			for _, msg := range batch {
				switch {
				case msg.isNotification():
					if glog.V(logger.Detail) {
						glog.Info("<-readResp: notification ", msg)
					}
					c.handleNotification(msg)
				case msg.isResponse():
					if glog.V(logger.Detail) {
						glog.Info("<-readResp: response ", msg)
					}
					c.handleResponse(msg)
				default:
					if glog.V(logger.Debug) {
						glog.Errorf("<-readResp: dropping weird message", msg)
					}
					// TODO: maybe close
				}
			}

		// Send path.
		case op := <-requestOpLock:
			// Establish the connection.
			// TODO: shit. It's too late to connect here because
			// we have already accepted the connection.
			var connectErr error
			if disconnected {
				connectErr = c.connectFunc()
			}
			// Stop listening for further send ops until the current one is done.
			requestOpLock = nil
			lastOp = op
			for _, id := range op.ids {
				c.respWait[string(id)] = op
			}

		case err := <-c.sendDone:
			if err != nil {
				// Remove response handlers for the last send. We'll probably exit soon
				// since a write failed. We remove those here because the error is already
				// handled in Call or BatchCall.
				for _, id := range lastOp.ids {
					delete(c.respWait, string(id))
				}
			}
			// Listen for send ops again.
			requestOpLock = c.requestOp
			lastOp = nil
		}
	}
}

// closeRequestOps unblocks pending send ops and active subscriptions.
func (c *Client) closeRequestOps(err error) {
	suberr := err
	if err == ErrClientQuit {
		// For subscriptions the close error is nil if the client was quit,
		// as documented for ClientSubscription.Err. This is because nil
		// is easier to handle as "don't re-establish".
		suberr = nil
	}

	didClose := make(map[*requestOp]bool)
	for _, op := range c.respWait {
		if !didClose[op] {
			if op.sub != nil {
				op.sub.quit <- err
			} else {
				op.err = err
				close(op.resp)
			}
			didClose[op] = true
		}
	}
	for _, sub := range c.subs {
		sub.quitWithError(suberr, false)
	}
}

func (c *Client) handleNotification(msg *jsonrpcMessage) {
	if msg.Method != notificationMethod {
		glog.V(logger.Debug).Info("dropping non-subscription message: ", msg)
		return
	}
	var subResult struct {
		ID     string          `json:"subscription"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(msg.Params, &subResult); err != nil {
		glog.V(logger.Debug).Info("dropping invalid subscription message: ", msg)
		return
	}
	if c.subs[subResult.ID] != nil {
		c.subs[subResult.ID].deliver(subResult.Result)
	}
}

func (c *Client) handleResponse(msg *jsonrpcMessage) {
	op := c.respWait[string(msg.ID)]
	if op == nil {
		glog.V(logger.Debug).Infof("unsolicited response %#v", msg)
		return
	}
	delete(c.respWait, string(msg.ID))
	// For normal responses, just forward the reply to Request/BatchRequest.
	if op.sub == nil {
		op.resp <- msg
		return
	}
	// For subscription responses, start the subscription if the server
	// indicates success. EthSubscribe gets unblocked in either case through
	// the quit channel.
	if msg.Error != nil {
		op.sub.quit <- msg.Error
		return
	}
	err := json.Unmarshal(msg.Result, &op.sub.subid)
	op.sub.quit <- err
	if err == nil {
		go op.sub.start()
		c.subs[op.sub.subid] = op.sub
	}
}

// Reading happens on a dedicated goroutine.

func (c *Client) read() error {
	defer c.wg.Done()
	var buf json.RawMessage
	for {
		resp, err := c.readMessage(&buf)
		if err != nil {
			c.readErr <- err
			return err
		}
		c.readResp <- resp
	}
}

func (c *Client) readMessage(buf *json.RawMessage) (rs []*jsonrpcMessage, err error) {
	if err = c.conn.Recv(buf); err != nil {
		return nil, err
	}
	if isBatch(*buf) {
		err = json.Unmarshal(*buf, &rs)
	} else {
		rs = make([]*jsonrpcMessage, 1)
		err = json.Unmarshal(*buf, &rs[0])
	}
	return rs, err
}

// Subscriptions.

// A ClientSubscription represents a subscription established through EthSubscribe.
type ClientSubscription struct {
	client  *Client
	etype   reflect.Type
	channel reflect.Value
	subid   string
	in      chan json.RawMessage
	// quit is dual-purpose. It is used to carry the response
	// status from dispatch to EthSubscribe before the subscription is
	// active. After it has been started, quit will be closed when the subscription
	// exits.
	quit chan error

	mu           sync.Mutex
	unsubscribed bool
	err          error
}

func newClientSubscription(c *Client, channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		client:  c,
		etype:   channel.Type().Elem(),
		channel: channel,
		// in is buffered so dispatch can continue even if the subscriber is slow.
		in: make(chan json.RawMessage, clientSubscriptionBuffer),
		// quit is buffered so dispatch can progress immediately while setting up the
		// subscription in handleResponse.
		quit: make(chan error, 1),
	}
	return sub
}

// Err returns the error that lead to the closing of the subscription channel.
//
// The intended use of Err is to schedule resubscription when the client connection is
// closed unexpectedly. After a call to Unsubscribe or Close on the underlying Client, Err
// will return nil.
func (sub *ClientSubscription) Err() error {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.err
}

// Unsubscribe unsubscribes the notification and closes the associated channel.
// It can safely be called more than once.
func (sub *ClientSubscription) Unsubscribe() {
	sub.quitWithError(nil, true)
}

func (sub *ClientSubscription) quitWithError(err error, unsubscribeServer bool) {
	sub.mu.Lock()
	// Keep the original error around.
	if sub.err == nil {
		sub.err = err
	}
	if !sub.unsubscribed {
		if unsubscribeServer {
			sub.requestUnsubscribe()
		}
		close(sub.quit)
		sub.channel.Close()
		sub.unsubscribed = true
	}
	sub.mu.Unlock()
}

func (sub *ClientSubscription) deliver(result json.RawMessage) (ok bool) {
	select {
	case sub.in <- result:
		return true
	case <-sub.quit:
		return false
	}
}

func (sub *ClientSubscription) start() {
	sub.quitWithError(sub.forward(), false)
}

func (sub *ClientSubscription) forward() error {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.quit)},
		{Dir: reflect.SelectSend, Chan: sub.channel},
	}
	for {
		select {
		case result := <-sub.in:
			val, err := sub.unmarshal(result)
			if err != nil {
				sub.requestUnsubscribe()
				return err
			}
			cases[1].Send = val
			switch chosen, _, _ := reflect.Select(cases); chosen {
			case 0: // <-sub.quit
				return nil
			case 1: // sub.channel<-
				continue
			}
		case <-sub.quit:
			return nil
		}
	}
}

func (sub *ClientSubscription) unmarshal(result json.RawMessage) (reflect.Value, error) {
	val := reflect.New(sub.etype)
	err := json.Unmarshal(result, val.Interface())
	return val.Elem(), err
}

func (sub *ClientSubscription) requestUnsubscribe() error {
	var result interface{}
	return sub.client.Call(&result, unsubscribeMethod, sub.subid)
}
