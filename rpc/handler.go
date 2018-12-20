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
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
)

// handler handles JSON-RPC messages. There is a handler for each connection.
type handler struct {
	reg           *serviceRegistry
	unsubscribeCb *callback

	respWait map[string]*requestOp          // active client requests
	subs     map[string]*ClientSubscription // active client subscriptions
}

func newHandler(reg *serviceRegistry) *handler {
	h := &handler{
		reg:      reg,
		respWait: make(map[string]*requestOp),
		subs:     make(map[string]*ClientSubscription),
	}
	h.unsubscribeCb = newCallback(reflect.Value{}, reflect.ValueOf(h.unsubscribe))
	return h
}

// handleBatch executes all messages in a batch and returns the responses.
func (h *handler) handleBatch(ctx context.Context, msgs []*jsonrpcMessage) []*jsonrpcMessage {
	answers := make([]*jsonrpcMessage, 0, len(msgs))
	for _, msg := range msgs {
		answer := h.handleMsg(ctx, msg)
		if answer != nil {
			answers = append(answers, answer)
		}
	}
	return answers
}

// handleMsg executes a single message and the returns the response.
func (h *handler) handleMsg(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	start := time.Now()
	switch {
	case msg.isNotification():
		h.handleNotification(ctx, msg)
		log.Trace("Handled notification", "method", msg.Method, "t", time.Since(start))
		return nil
	case msg.isResponse():
		h.handleResponse(msg)
		log.Trace("Handled RPC response", "id", string(msg.ID), "t", time.Since(start))
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

// handleNotification processes method calls that don't need a response.
func (h *handler) handleNotification(ctx context.Context, msg *jsonrpcMessage) {
	if !strings.HasSuffix(msg.Method, notificationMethodSuffix) {
		h.handleCall(ctx, msg)
		return
	}

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
