package rpc

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ethereum/go-ethereum/log"
)

// A value of this type can a JSON-RPC request, notification, successful response or
// error response. Which one it is depends on the fields.
type jsonrpcMessage struct {
	Version string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Error   *jsonError      `json:"error,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
}

func (msg *jsonrpcMessage) isNotification() bool {
	return msg.ID == nil && msg.Method != ""
}

func (msg *jsonrpcMessage) isCall() bool {
	return msg.hasValidID() && msg.Method != ""
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

func (h *handler) handleBatch(ctx context.Context, msgs []*jsonrpcMessage) {
	answers := make([]*jsonrpcMessage, 0, len(msgs))
	for _, msg := range msgs {
		answer := h.execMsg(ctx, msg)
		if answer != nil {
			answers = append(answers, answer)
		}
	}
	h.send(answers)
}

// execMsg executes a single message.
func (h *handler) execMsg(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	switch {
	case msg.isNotification():
		log.Trace("Received notification", "msg", msg)
		return c.handleNotification(ctx, msg)
	case msg.isResponse():
		log.Trace("Received response", "msg", msg)
		return c.handleResponse(ctx, msg)
	case msg.isCall():
		log.Trace("Received call", "msg", msg)
		return c.handleCall(ctx, msg)
	default:
		log.Debug("Dropping weird RPC message", "msg", msg)
		return nil
	}
}

func (h *handler) handleCall(ctx context.Context, msg *jsonrpcMessage) *jsonrpcMessage {
	if isUnsubscribe(msg) { // cancel subscription, first param must be the subscription id
		if len(req.args) >= 1 && req.args[0].Kind() == reflect.String {
			notifier, supported := NotifierFromContext(ctx)
			if !supported { // interface doesn't support subscriptions (e.g. http)
				return codec.CreateErrorResponse(&req.id, &callbackError{ErrNotificationsUnsupported.Error()}), nil
			}

			subid := ID(req.args[0].String())
			if err := notifier.unsubscribe(subid); err != nil {
				return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
			}

			return codec.CreateResponse(req.id, true), nil
		}
		return codec.CreateErrorResponse(&req.id, &invalidParamsError{"Expected subscription id as first argument"}), nil
	}
	if req.callb.isSubscribe {
		subid, err := s.createSubscription(ctx, codec, req)
		if err != nil {
			return codec.CreateErrorResponse(&req.id, &callbackError{err.Error()}), nil
		}

		// active the subscription after the sub id was successfully sent to the client
		activateSub := func() {
			notifier, _ := NotifierFromContext(ctx)
			notifier.activate(subid, req.svcname)
		}

		return codec.CreateResponse(req.id, subid), activateSub
	}

	// regular RPC call, prepare arguments
	if len(req.args) != len(req.callb.argTypes) {
		rpcErr := &invalidParamsError{fmt.Sprintf("%s%s%s expects %d parameters, got %d",
			req.svcname, serviceMethodSeparator, req.callb.method.Name,
			len(req.callb.argTypes), len(req.args))}
		return codec.CreateErrorResponse(&req.id, rpcErr), nil
	}

	arguments := []reflect.Value{req.callb.rcvr}
	if req.callb.hasCtx {
		arguments = append(arguments, reflect.ValueOf(ctx))
	}
	if len(req.args) > 0 {
		arguments = append(arguments, req.args...)
	}

	// execute RPC method and return result
	reply := req.callb.method.Func.Call(arguments)
	if len(reply) == 0 {
		return codec.CreateResponse(req.id, nil), nil
	}
	if req.callb.errPos >= 0 { // test if method returned an error
		if !reply[req.callb.errPos].IsNil() {
			e := reply[req.callb.errPos].Interface().(error)
			res := codec.CreateErrorResponse(&req.id, &callbackError{e.Error()})
			return res, nil
		}
	}
	return codec.CreateResponse(req.id, reply[0].Interface()), nil
}
