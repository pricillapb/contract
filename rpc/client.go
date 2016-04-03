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

type Client struct {
	conn      ClientCodec
	idCounter uint32

	// for dispatch
	didQuit      chan struct{}
	readErr      chan error
	readResp     chan []jsonrpcMessage
	handlerOp    chan handlerOp
	respHandlers map[string]chan<- *jsonrpcMessage
	subHandlers  map[string]chan<- *jsonrpcMessage
}

const (
	// Handler Set Operations.
	addRespHandler = iota
	delRespHandler
	addSubHandler
	delSubHandler
)

type handlerOp struct {
	op      uint8
	id      string
	channel chan<- *jsonrpcMessage
}

func NewClient(codec ClientCodec) *Client {
	c := &Client{
		conn:         codec,
		didQuit:      make(chan struct{}),
		readErr:      make(chan error),
		readResp:     make(chan []jsonrpcMessage),
		handlerOp:    make(chan handlerOp),
		respHandlers: make(map[string]chan<- *jsonrpcMessage),
		subHandlers:  make(map[string]chan<- *jsonrpcMessage),
	}
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
}

// Request performs a JSON-RPC call with the given arguments
// and unmarshals into result if no error occurred.
func (c *Client) Request(result interface{}, method string, args ...interface{}) error {
	params, err := json.Marshal(args)
	if err != nil {
		return err
	}
	req := &jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: method, Params: params}
	respCh := make(chan *jsonrpcMessage, 1)
	if !c.doHandlerOp(addRespHandler, string(req.ID), respCh) {
		return ErrClientQuit
	}
	if err := c.conn.Send(req); err != nil {
		c.doHandlerOp(delRespHandler, string(req.ID), nil)
		return err
	}
	// loop has accepted the handler and will close the channel it when it quits.
	resp, isopen := <-respCh
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
	msgs := make([]jsonrpcMessage, len(b))
	respCh := make(chan *jsonrpcMessage, len(b))
	for i, elem := range b {
		params, err := json.Marshal(elem.Args)
		if err != nil {
			return err
		}
		msgs[i] = jsonrpcMessage{Version: "2.0", ID: c.nextID(), Method: elem.Method, Params: params}
		if !c.doHandlerOp(addRespHandler, string(msgs[i].ID), respCh) {
			return ErrClientQuit
		}
	}
	if err := c.conn.Send(msgs); err != nil {
		for _, msg := range msgs {
			c.doHandlerOp(delRespHandler, string(msg.ID), nil)
		}
		return err
	}
	// dispatch has accepted the handlers and will close respCh when it quits.
	for n := 0; n < len(b); n++ {
		resp, isopen := <-respCh
		if !isopen {
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
// The channel is closed when the notification is unsubscribed.
// If an error occurred, it can be retrieved via the Err method of the subscription.
func (c *Client) EthSubscribe(channel interface{}, args ...interface{}) (*ClientSubscription, error) {
	chanVal := reflect.ValueOf(channel)
	if chanVal.Kind() != reflect.Chan {
		panic("first argument to EthSubscribe must be channel-typed")
	}
	if chanVal.IsNil() {
		panic("channel given to EthSubscribe must not be nil")
	}

	// Start the subscription on the server side.
	var subid string
	if err := c.Request(&subid, subscribeMethod, args...); err != nil {
		return nil, err
	}
	sub := newClientSubscription(subid, chanVal)
	return sub, nil
}

func (c *Client) doHandlerOp(op uint8, id string, ch chan<- *jsonrpcMessage) (ok bool) {
	select {
	case c.handlerOp <- handlerOp{op, id, ch}:
		return true
	case <-c.didQuit:
		return false
	}
}

// dispatch is the main loop of the client.
// It sends read messages to waiting calls to Request and BatchRequest
// and subscription notifications to registered subscriptions.
func (c *Client) dispatch() {
	defer c.conn.Close()
	defer func() {
		// Notify pending handlers on exit.
		for _, ch := range c.respHandlers {
			close(ch)
		}
		// TODO: subscriptions.
		close(c.didQuit)
	}()

	for {
		select {
		case err := <-c.readErr:
			glog.V(logger.Debug).Infof("<-readErr: shutting down (%v)", err)
			return
		case batch := <-c.readResp:
			for _, msg := range batch {
				switch {
				case msg.isNotification():
					c.handleNotification(msg)
				case msg.isResponse():
					c.handleResponse(msg)
				default:
					glog.V(logger.Debug).Infof("dropping weird message %#v", msg)
					// TODO: maybe close
				}
			}
		case h := <-c.handlerOp:
			switch h.op {
			case addRespHandler:
				c.respHandlers[h.id] = h.channel
			case delRespHandler:
				delete(c.respHandlers, h.id)
			case addSubHandler:
				c.subHandlers[h.id] = h.channel
			case delSubHandler:
				delete(c.subHandlers, h.id)
			default:
				panic("invalid handler action")
			}
		}
	}
}

func (c *Client) handleNotification(msg jsonrpcMessage) {
	if msg.Method != notificationMethod {
		glog.V(logger.Debug).Infof("dropping non-subscription notification %#v", msg)
		return
	}
	var sub jsonSubscription
	if err := json.Unmarshal(msg.Result, &sub); err != nil {
		glog.V(logger.Debug).Infof("dropping invalid subscription notification %#v", msg)
		return
	}
	if c.subHandlers[sub.Subscription] != nil {
		c.subHandlers[sub.Subscription] <- &msg
	}
}

func (c *Client) handleResponse(msg jsonrpcMessage) {
	hchan := c.respHandlers[string(msg.ID)]
	if hchan != nil {
		hchan <- &msg
		delete(c.respHandlers, string(msg.ID))
	}
}

// Reading happens on a dedicated goroutine.

func (c *Client) read() error {
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

func (c *Client) readMessage(buf *json.RawMessage) (rs []jsonrpcMessage, err error) {
	if err = c.conn.Recv(buf); err != nil {
		return nil, err
	}
	if len(*buf) > 0 && (*buf)[0] == '[' {
		err = json.Unmarshal(*buf, &rs)
	} else {
		rs = make([]jsonrpcMessage, 1)
		err = json.Unmarshal(*buf, &rs[0])
	}
	return rs, err
}

// Subscriptions.

// A ClientSubscription represents a subscription established through EthSubscribe.
type ClientSubscription struct {
	channel reflect.Value
	subid   string
	in      chan *jsonrpcMessage
	quit    chan struct{}

	mu           sync.Mutex
	unsubscribed bool
	err          error
}

func newClientSubscription(subid string, channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		channel: channel,
		subid:   subid,
		in:      make(chan *jsonrpcMessage, clientSubscriptionBuffer),
		quit:    make(chan struct{}),
	}
	go sub.forward()
	return sub
}

// Unsubscribe unsubscribes the notification and closes the associated channel.
// It can safely be called more than once.
func (sub *ClientSubscription) Unsubscribe() {
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

func (sub *ClientSubscription) deliver(notification *jsonrpcMessage) (ok bool) {
	select {
	case sub.in <- notification:
		return true
	case <-sub.quit:
		return false
	}
}

func (sub *ClientSubscription) forward() {
	err := sub.forwardLoop()
	sub.mu.Lock()
	sub.err = err
	if !sub.unsubscribed {
		close(sub.quit)
		sub.unsubscribed = true
	}
	sub.mu.Unlock()
}

func (sub *ClientSubscription) forwardLoop() error {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.quit)},
		{Dir: reflect.SelectSend, Chan: sub.channel},
	}
	for {
		select {
		case msg := <-sub.in:
			val, err := sub.unmarshal(msg)
			if err != nil {
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

func (sub *ClientSubscription) unmarshal(notification *jsonrpcMessage) (reflect.Value, error) {
	// First unmarshal the notification itself.
}
