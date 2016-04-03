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
	"encoding/json"
	"errors"
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

type ClientCodec interface {
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
	Version string          `json:"version"`
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
	return msg.hasValidID() && (len(msg.Result) > 0 || msg.Error != nil)
}

func (msg *jsonrpcMessage) hasValidID() bool {
	return len(msg.ID) > 0 && msg.ID[0] != '{' && msg.ID[0] != '['
}

func (msg *jsonrpcMessage) String() string {
	b, _ := json.Marshal(msg)
	return string(b)
}

type Client struct {
	conn      ClientCodec
	wg        sync.WaitGroup
	idCounter uint32

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
	ids  []json.RawMessage
	resp chan *jsonrpcMessage // set for requests, batch requests
	sub  *ClientSubscription  // set for subscriptions
}

func NewClient(codec ClientCodec) *Client {
	c := &Client{
		conn:      codec,
		didQuit:   make(chan struct{}),
		readErr:   make(chan error),
		readResp:  make(chan []*jsonrpcMessage),
		requestOp: make(chan *requestOp),
		sendDone:  make(chan error),
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

// Close closes the client, aborting any in-flight requests.
func (c *Client) Close() {
	c.conn.Close()
	c.wg.Wait()
}

// Request performs a JSON-RPC call with the given arguments
// and unmarshals into result if no error occurred.
func (c *Client) Request(result interface{}, method string, args ...interface{}) error {
	msg, err := c.newMessage(method, args...)
	if err != nil {
		return err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, resp: make(chan *jsonrpcMessage, 1)}
	if err := c.send(op, msg); err != nil {
		return err
	}
	// dispatch has accepted the request and will close the channel it when it quits.
	resp, isopen := <-op.resp
	if !isopen {
		return ErrClientQuit
	} else if resp.Error != nil {
		return resp.Error
	}
	// Don't unmarshal if the caller is not interested.
	if result == nil {
		return nil
	}
	if len(resp.Result) == 0 {
		return ErrNoResult
	}
	return json.Unmarshal(resp.Result, result)
}

// BatchRequest sends all given requests as a single batch
// and waits for the server to return a response for all of them.
//
// In contrast to Request, BatchRequest only returns I/O errors.
// Any error specific to a request is reported through the Error field
// of the corresponding BatchElem.
func (c *Client) BatchRequest(b []BatchElem) error {
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
	if err := c.send(op, msgs); err != nil {
		return err
	}
	// dispatch has accepted the handlers and will close respCh when it quits.
	for n := 0; n < len(b); n++ {
		resp, isopen := <-op.resp
		if !isopen {
			// TODO: not right. would be nicer to return the actual read error,
			// if any, but that's not easy to get.
			return ErrClientQuit
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
		if elem.Result != nil {
			if len(resp.Result) == 0 {
				elem.Error = ErrNoResult
				continue
			}
			elem.Error = json.Unmarshal(resp.Result, elem.Result)
		}
	}
	return nil
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
	chanVal := reflect.ValueOf(channel)
	if chanVal.Kind() != reflect.Chan {
		panic("first argument to EthSubscribe must be channel-typed")
	}
	if chanVal.IsNil() {
		panic("channel given to EthSubscribe must not be nil")
	}

	msg, err := c.newMessage(subscribeMethod, args...)
	if err != nil {
		return nil, err
	}
	op := &requestOp{ids: []json.RawMessage{msg.ID}, sub: &ClientSubscription{channel: chanVal}}
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
	)
	for {
		select {
		// Read path.
		case err := <-c.readErr:
			glog.V(logger.Debug).Infof("<-readErr: shutting down (%v)", err)
			return
		case batch := <-c.readResp:
			for _, msg := range batch {
				switch {
				case msg.isNotification():
					if glog.V(logger.Detail) {
						glog.Info("<-readResp: got notification ", msg)
					}
					c.handleNotification(msg)
				case msg.isResponse():
					if glog.V(logger.Detail) {
						glog.Info("<-readResp: got response ", msg)
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
			// Stop listening for further send ops until the current one is done.
			requestOpLock = nil
			lastOp = op
			for _, id := range op.ids {
				c.respWait[string(id)] = op
			}
		case err := <-c.sendDone:
			if err != nil {
				// Remove response handlers for the last send. We'll probably exit soon
				// since a write failed.
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

// closeRequestOps unblocks pending send ops and active subscriptions on exit.
func (c *Client) closeRequestOps(err error) {
	didClose := make(map[*requestOp]bool)
	for _, op := range c.respWait {
		if !didClose[op] {
			// TODO: maybe assign error instead of sending it.
			if op.sub != nil {
				op.sub.quit <- err
			} else {
				close(op.resp)
			}
			didClose[op] = true
		}
	}
	for _, sub := range c.subs {
		sub.closeWithError(ErrClientQuit)
	}
}

func (c *Client) handleNotification(msg *jsonrpcMessage) {
	if msg.Method != notificationMethod {
		glog.V(logger.Debug).Infof("dropping non-subscription notification %#v", msg)
		return
	}
	var subResult struct {
		ID     string          `json:"subscription"`
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(msg.Result, &subResult); err != nil {
		glog.V(logger.Debug).Infof("dropping invalid subscription notification %#v", msg)
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
	err := json.Unmarshal(msg.Result, op.sub.subid)
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
	if len(*buf) > 0 && (*buf)[0] == '[' {
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

func newClientSubscription(channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		etype:   channel.Type().Elem(),
		channel: channel,
		in:      make(chan json.RawMessage, clientSubscriptionBuffer),
		quit:    make(chan error),
	}
	return sub
}

// Unsubscribe unsubscribes the notification and closes the associated channel.
// It can safely be called more than once.
func (sub *ClientSubscription) Unsubscribe() {
	// TODO: send eth_unsubscribe.
	sub.mu.Lock()
	defer sub.mu.Unlock()
	if !sub.unsubscribed {
		close(sub.quit)
		sub.unsubscribed = true
	}
}

// Err returns the error that lead to the closing of the subscription channel.
// Ordinary calls to Unsubscribe leave Err blank.
func (sub *ClientSubscription) Err() error {
	sub.mu.Lock()
	defer sub.mu.Unlock()
	return sub.err
}

func (sub *ClientSubscription) deliver(result json.RawMessage) (ok bool) {
	select {
	case sub.in <- result:
		return true
	case <-sub.quit:
		return false
	}
}

func (sub *ClientSubscription) closeWithError(err error) {
	sub.mu.Lock()
	sub.err = err
	if !sub.unsubscribed {
		close(sub.quit)
		sub.unsubscribed = true
	}
	sub.mu.Unlock()
}

func (sub *ClientSubscription) start() {
	sub.closeWithError(sub.forward())
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
				// TODO: send eth_unsubscribe
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
	return val, err
}
