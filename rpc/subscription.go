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
	"bufio"
	crand "crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math/rand"
	"strings"
	"sync"
	"time"
)

var (
	// ErrNotificationsUnsupported is returned when the connection doesn't support notifications
	ErrNotificationsUnsupported = errors.New("notifications not supported")
	// ErrNotificationNotFound is returned when the notification for the given id is not found
	ErrSubscriptionNotFound = errors.New("subscription not found")
)

var (
	subscriptionIDGenMu sync.Mutex
	subscriptionIDGen   = idGenerator()
)

// ID defines a pseudo random number that is used to identify RPC subscriptions.
type ID string

// idGenerator helper utility that generates a (pseudo) random sequence of
// bytes that are used to generate identifiers.
func idGenerator() *rand.Rand {
	if seed, err := binary.ReadVarint(bufio.NewReader(crand.Reader)); err == nil {
		return rand.New(rand.NewSource(seed))
	}
	return rand.New(rand.NewSource(int64(time.Now().Nanosecond())))
}

// NewID generates a identifier that can be used as an identifier in the RPC interface.
// e.g. filter and subscription identifier.
func NewID() ID {
	subscriptionIDGenMu.Lock()
	defer subscriptionIDGenMu.Unlock()

	id := make([]byte, 16)
	for i := 0; i < len(id); i += 7 {
		val := subscriptionIDGen.Int63()
		for j := 0; i+j < len(id) && j < 7; j++ {
			id[i+j] = byte(val)
			val >>= 8
		}
	}

	rpcId := hex.EncodeToString(id)
	// rpc ID's are RPC quantities, no leading zero's and 0 is 0x0
	rpcId = strings.TrimLeft(rpcId, "0")
	if rpcId == "" {
		rpcId = "0"
	}

	return ID("0x" + rpcId)
}

// a Subscription is created by a notifier and tight to that notifier. The client can use
// this subscription to wait for an unsubscribe request for the client, see Err().
type Subscription struct {
	ID        ID
	namespace string
	err       chan error // closed on unsubscribe
}

// Err returns a channel that is closed when the client send an unsubscribe request.
func (s *Subscription) Err() <-chan error {
	return s.err
}

// MarshalJSON marshals a subscription as its ID.
func (s *Subscription) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.ID)
}

type serverNotifier struct {
	codec ServerCodec

	subMu    sync.Mutex
	active   map[ID]*Subscription
	inactive []*Subscription
	// Unsent notifications of inactive subscriptions are buffered until
	// activated. If buffer is nil, activation has already happened and no
	// further subscriptions can be created.
	buffer map[ID][]interface{}
}

// serverNotifierKey is used to store the active serverNotifier in the request context.
type serverNotifierKey struct{}

// newNotifier creates a new notifier that can be used to send subscription
// notifications to the client.
func newServerNotifier(codec ServerCodec) *serverNotifier {
	return &serverNotifier{
		codec:  codec,
		active: make(map[ID]*Subscription),
		buffer: make(map[ID][]interface{}),
	}
}

func (n *serverNotifier) createSubscription(namespace string) *Subscription {
	s := &Subscription{ID: NewID(), namespace: namespace, err: make(chan error)}
	n.subMu.Lock()
	alreadyActivated := n.buffer == nil
	if !alreadyActivated {
		n.inactive = append(n.inactive, s)
	}
	n.subMu.Unlock()
	if alreadyActivated {
		panic("rpc: call to CreateSubscription after RPC method has returned")
	}
	return s
}

// subscriptionNotify sends a notification to the client with the given data as payload.
// If an error occurs the RPC connection is closed and the error is returned.
func (n *serverNotifier) subscriptionNotify(id ID, data interface{}) error {
	n.subMu.Lock()
	defer n.subMu.Unlock()

	if sub, active := n.active[id]; active {
		n.send(sub, data)
	} else {
		n.buffer[id] = append(n.buffer[id], data)
	}
	return nil
}

func (n *serverNotifier) send(sub *Subscription, data interface{}) error {
	enc, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return n.codec.Write(subscriptionNotification(sub.namespace, sub.ID, enc))
}

// unsubscribe a subscription.
// If the subscription could not be found ErrSubscriptionNotFound is returned.
func (n *serverNotifier) unsubscribe(id ID) error {
	n.subMu.Lock()
	defer n.subMu.Unlock()
	if s, found := n.active[id]; found {
		close(s.err)
		delete(n.active, id)
		return nil
	}
	return ErrSubscriptionNotFound
}

// activate enables all subscriptions. Until a subscription is enabled all
// notifications are dropped. This method is called by the RPC server after
// the subscription ID was sent to client. This prevents notifications being
// sent to the client before the subscription ID is sent to the client.
func (n *serverNotifier) activate() {
	n.subMu.Lock()
	defer n.subMu.Unlock()

	for _, sub := range n.inactive {
		// TODO: set namespace
		n.active[sub.ID] = sub
		// Send buffered notifications.
		for _, data := range n.buffer[sub.ID] {
			n.send(sub, data)
		}
	}
	n.buffer = nil
}
