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

package rpc

import (
	"container/list"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

// handler handles JSON-RPC messages. There is a handler for each connection.
type handler struct {
	reg           *serviceRegistry
	unsubscribeCb *callback
	respWait      map[string]*requestOp          // active client requests
	subs          map[string]*ClientSubscription // active client subscriptions

	calls      sync.WaitGroup
	rootCtx    context.Context
	cancelRoot func()
	conn       jsonWriter // where responses will be sent

	allowSubscriptions bool
	subscriptionIDgen  func() ID
}

func newHandler(conn jsonWriter, reg *serviceRegistry) *handler {
	rootCtx, cancelRoot := context.WithCancel(context.Background())
	h := &handler{
		reg:               reg,
		conn:              conn,
		respWait:          make(map[string]*requestOp),
		subs:              make(map[string]*ClientSubscription),
		rootCtx:           rootCtx,
		cancelRoot:        cancelRoot,
		subscriptionIDgen: randomIDGenerator(),
	}
	h.unsubscribeCb = newCallback(reflect.Value{}, reflect.ValueOf(h.unsubscribe))
	return h
}

// close cancels all requests and waits for call goroutines to shut down.
func (h *handler) close(err error) {
	h.cancelAllRequests(err)
	h.cancelRoot()
	h.calls.Wait()
}

// startCallProc runs fn in a new goroutine and starts tracking it in the h.calls wait group.
func (h *handler) startCallProc(fn func(context.Context)) {
	h.calls.Add(1)
	go func() {
		ctx, cancel := context.WithCancel(h.rootCtx)
		defer h.calls.Done()
		defer cancel()
		if h.allowSubscriptions {
			sn := newServerNotifier(h.subscriptionIDgen, h.conn)
			ctx = context.WithValue(ctx, serverNotifierKey{}, sn)
			defer sn.activate()
		}
		fn(ctx)
	}()
}

// handleBatch executes all messages in a batch and returns the responses.
func (h *handler) handleBatch(msgs []*jsonrpcMessage) {
	// Handle non-call messages first:
	calls := make([]*jsonrpcMessage, 0, len(msgs))
	for _, msg := range msgs {
		if handled := h.handleImmediate(msg); !handled {
			calls = append(calls, msg)
		}
	}
	if len(calls) == 0 {
		return
	}
	// Process calls on a goroutine because they may block indefinitely:
	h.startCallProc(func(ctx context.Context) {
		answers := make([]*jsonrpcMessage, 0, len(msgs))
		for _, msg := range calls {
			if answer := h.handleCallMsg(ctx, msg); answer != nil {
				answers = append(answers, answer)
			}
		}
		if len(answers) > 0 {
			h.conn.Write(answers)
		}
	})
}

// handleMsg handles a single message.
func (h *handler) handleMsg(msg *jsonrpcMessage) {
	if ok := h.handleImmediate(msg); ok {
		return
	}
	h.startCallProc(func(ctx context.Context) {
		if answer := h.handleCallMsg(ctx, msg); answer != nil {
			h.conn.Write(answer)
		}
	})
}

// handleImmediate executes non-call messages. It returns false if the message is a call.
func (h *handler) handleImmediate(msg *jsonrpcMessage) bool {
	start := time.Now()
	switch {
	case msg.isNotification():
		ok := strings.HasSuffix(msg.Method, notificationMethodSuffix)
		if ok {
			h.handleSubscriptionResult(msg)
		}
		return ok
	case msg.isResponse():
		h.handleResponse(msg)
		log.Trace("Handled RPC response", "id", string(msg.ID), "t", time.Since(start))
		return true
	default:
		return false
	}
}

// handleCallMsg executes a call message and returns the answer.
func (h *handler) handleCallMsg(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	start := time.Now()
	switch {
	case msg.isNotification():
		h.handleCall(ctx, msg)
		log.Debug("Served "+msg.Method, "t", time.Since(start))
		return nil
	case msg.isCall():
		resp := h.handleCall(ctx, msg)
		log.Debug("Served "+msg.Method, "id", string(msg.ID), "t", time.Since(start))
		return resp
	case msg.hasValidID():
		return msg.errorResponse(&invalidRequestError{"invalid request"})
	default:
		return errorMessage(&invalidRequestError{"invalid request"})
	}
}

// handleSubscriptionResult processes subscription notifications.
func (h *handler) handleSubscriptionResult(msg *jsonrpcMessage) {
	var result subscriptionResult
	if err := json.Unmarshal(msg.Params, &result); err != nil {
		log.Debug("dropping invalid subscription message", "msg", msg)
		return
	}
	if h.subs[result.ID] != nil {
		h.subs[result.ID].deliver(result.Result)
	}
}

// handleResponse processes method call responses.
func (h *handler) handleResponse(msg *jsonrpcMessage) {
	op := h.respWait[string(msg.ID)]
	if op == nil {
		log.Debug("unsolicited response", "msg", msg)
		return
	}
	delete(h.respWait, string(msg.ID))
	// For normal responses, just forward the reply to Call/BatchCall.
	if op.sub == nil {
		op.resp <- msg
		return
	}
	// For subscription responses, start the subscription if the server
	// indicates success. EthSubscribe gets unblocked in either case through
	// the op.resp channel.
	defer close(op.resp)
	if msg.Error != nil {
		op.err = msg.Error
		return
	}
	if op.err = json.Unmarshal(msg.Result, &op.sub.subid); op.err == nil {
		go op.sub.start()
		h.subs[op.sub.subid] = op.sub
	}
}

// handleCall processes method calls.
func (h *handler) handleCall(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	if msg.isSubscribe() {
		return h.handleSubscribe(ctx, msg)
	}
	var callb *callback
	if msg.isUnsubscribe() {
		callb = h.unsubscribeCb
	} else {
		callb = h.reg.callback(msg.Method)
	}
	if callb == nil {
		return msg.errorResponse(&methodNotFoundError{method: msg.Method})
	}
	args, err := parsePositionalArguments(msg.Params, callb.argTypes)
	if err != nil {
		return msg.errorResponse(&invalidParamsError{err.Error()})
	}

	return h.runMethod(ctx, msg, callb, args)
}

// handleSubscribe processes *_subscribe method calls.
func (h *handler) handleSubscribe(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	sn, ok := ctx.Value(serverNotifierKey{}).(*serverNotifier)
	if !ok {
		return msg.errorResponse(ErrNotificationsUnsupported)
	}

	// Subscription method name is first argument.
	name, err := parseSubscriptionName(msg.Params)
	if err != nil {
		return msg.errorResponse(&invalidParamsError{err.Error()})
	}
	namespace := msg.namespace()
	callb := h.reg.subscription(namespace, name)
	if callb == nil {
		return msg.errorResponse(&subscriptionNotFoundError{namespace, name})
	}

	// Parse subscription name arg too, but remove it before calling the callback.
	argTypes := append([]reflect.Type{stringType}, callb.argTypes...)
	args, err := parsePositionalArguments(msg.Params, argTypes)
	if err != nil {
		return msg.errorResponse(&invalidParamsError{err.Error()})
	}
	args = args[1:]

	// Install notifier in context so the subscription handler can find it.
	n := &Notifier{namespace, sn}
	ctx = context.WithValue(ctx, notifierKey{}, n)

	return h.runMethod(ctx, msg, callb, args)
}

// runMethod runs the Go callback for an RPC method.
func (h *handler) runMethod(ctx context.Context, msg *jsonrpcMessage, callb *callback, args []reflect.Value) *jsonrpcMessage {
	result, err := callb.call(ctx, msg.Method, args)
	if err != nil {
		return msg.errorResponse(err)
	}
	return msg.response(result)
}

// unsubscribe is the callback function for all *_unsubscribe calls.
func (h *handler) unsubscribe(ctx context.Context, subid ID) (bool, error) {
	sn, ok := ctx.Value(serverNotifierKey{}).(*serverNotifier)
	if !ok {
		return false, ErrNotificationsUnsupported
	}
	if err := sn.unsubscribe(subid); err != nil {
		return false, err
	}
	return true, nil
}

// addRequestOp registers a request operation.
func (h *handler) addRequestOp(op *requestOp) {
	for _, id := range op.ids {
		h.respWait[string(id)] = op
	}
}

// removeRequestOps stops waiting for the given request IDs.
func (h *handler) removeRequestOp(op *requestOp) {
	for _, id := range op.ids {
		delete(h.respWait, string(id))
	}
}

// cancelAllRequests unblocks pending requests ops and active subscriptions.
func (h *handler) cancelAllRequests(err error) {
	didClose := make(map[*requestOp]bool)

	for id, op := range h.respWait {
		// Remove the op so that later calls will not close op.resp again.
		delete(h.respWait, id)

		if !didClose[op] {
			op.err = err
			close(op.resp)
			didClose[op] = true
		}
	}
	for id, sub := range h.subs {
		delete(h.subs, id)
		sub.quitWithError(err, false)
	}
}

// Subscriptions.

// A ClientSubscription represents a subscription established through EthSubscribe.
type ClientSubscription struct {
	client    *Client
	etype     reflect.Type
	channel   reflect.Value
	namespace string
	subid     string
	in        chan json.RawMessage

	quitOnce sync.Once     // ensures quit is closed once
	quit     chan struct{} // quit is closed when the subscription exits
	errOnce  sync.Once     // ensures err is closed once
	err      chan error
}

func newClientSubscription(c *Client, namespace string, channel reflect.Value) *ClientSubscription {
	sub := &ClientSubscription{
		client:    c,
		namespace: namespace,
		etype:     channel.Type().Elem(),
		channel:   channel,
		quit:      make(chan struct{}),
		err:       make(chan error, 1),
		in:        make(chan json.RawMessage),
	}
	return sub
}

// Err returns the subscription error channel. The intended use of Err is to schedule
// resubscription when the client connection is closed unexpectedly.
//
// The error channel receives a value when the subscription has ended due
// to an error. The received error is nil if Close has been called
// on the underlying client and no other error has occurred.
//
// The error channel is closed when Unsubscribe is called on the subscription.
func (sub *ClientSubscription) Err() <-chan error {
	return sub.err
}

// Unsubscribe unsubscribes the notification and closes the error channel.
// It can safely be called more than once.
func (sub *ClientSubscription) Unsubscribe() {
	sub.quitWithError(nil, true)
	sub.errOnce.Do(func() { close(sub.err) })
}

func (sub *ClientSubscription) quitWithError(err error, unsubscribeServer bool) {
	sub.quitOnce.Do(func() {
		// The dispatch loop won't be able to execute the unsubscribe call
		// if it is blocked on deliver. Close sub.quit first because it
		// unblocks deliver.
		close(sub.quit)
		if unsubscribeServer {
			sub.requestUnsubscribe()
		}
		if err != nil {
			if err == ErrClientQuit {
				err = nil // Adhere to subscription semantics.
			}
			sub.err <- err
		}
	})
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
	sub.quitWithError(sub.forward())
}

func (sub *ClientSubscription) forward() (err error, unsubscribeServer bool) {
	cases := []reflect.SelectCase{
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.quit)},
		{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(sub.in)},
		{Dir: reflect.SelectSend, Chan: sub.channel},
	}
	buffer := list.New()
	defer buffer.Init()
	for {
		var chosen int
		var recv reflect.Value
		if buffer.Len() == 0 {
			// Idle, omit send case.
			chosen, recv, _ = reflect.Select(cases[:2])
		} else {
			// Non-empty buffer, send the first queued item.
			cases[2].Send = reflect.ValueOf(buffer.Front().Value)
			chosen, recv, _ = reflect.Select(cases)
		}

		switch chosen {
		case 0: // <-sub.quit
			return nil, false
		case 1: // <-sub.in
			val, err := sub.unmarshal(recv.Interface().(json.RawMessage))
			if err != nil {
				return err, true
			}
			if buffer.Len() == maxClientSubscriptionBuffer {
				return ErrSubscriptionQueueOverflow, true
			}
			buffer.PushBack(val)
		case 2: // sub.channel<-
			cases[2].Send = reflect.Value{} // Don't hold onto the value.
			buffer.Remove(buffer.Front())
		}
	}
}

func (sub *ClientSubscription) unmarshal(result json.RawMessage) (interface{}, error) {
	val := reflect.New(sub.etype)
	err := json.Unmarshal(result, val.Interface())
	return val.Elem().Interface(), err
}

func (sub *ClientSubscription) requestUnsubscribe() error {
	var result interface{}
	return sub.client.Call(&result, sub.namespace+unsubscribeMethodSuffix, sub.subid)
}
