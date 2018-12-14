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
	"strings"
	"testing"
)

/*

func TestNotifications(t *testing.T) {
	server := NewServer()
	service := &NotificationTestService{unsubscribed: make(chan string)}

	if err := server.RegisterName("eth", service); err != nil {
		t.Fatalf("unable to register test service %v", err)
	}

	clientConn, serverConn := net.Pipe()
	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)

	out := json.NewEncoder(clientConn)
	in := json.NewDecoder(clientConn)

	n := 5
	val := 12345
	request := map[string]interface{}{
		"id":      1,
		"method":  "eth_subscribe",
		"version": "2.0",
		"params":  []interface{}{"someSubscription", n, val},
	}

	// create subscription
	if err := out.Encode(request); err != nil {
		t.Fatal(err)
	}

	var response map[string]interface{}
	if err := in.Decode(&response); err != nil {
		t.Fatal(err)
	}
	var ok bool
	if _, ok = response["result"].(string); !ok {
		t.Fatalf("expected subscription id, got %+v", response)
	}

	type notificationResult struct{
		t.Fatalf("expected subscription")
	}

	for i := 0; i < n; i++ {
		var notification map[string]interface{}
		if err := in.Decode(&notification); err != nil {
			t.Fatalf("%v", err)
		}
		sv := int(notification["params"].([]interface{})[0].(map[string]interface{})["result"].(float64))
		if sv != val+i {
			t.Fatalf("expected %d, got %d", val+i, sv)
		}
	}

	clientConn.Close() // causes notification unsubscribe callback to be called
	select {
	case <-service.unsubscribed:
	case <-time.After(1 * time.Second):
		t.Fatal("Unsubscribe not called after one second")
	}
}


// TestSubscriptionMultipleNamespaces ensures that subscriptions can exists
// for multiple different namespaces.
func TestSubscriptionMultipleNamespaces(t *testing.T) {
	var (
		namespaces        = []string{"eth", "shh", "bzz"}
		service           = NotificationTestService{}
		subCount          = len(namespaces) * 2
		notificationCount = 3

		server                 = NewServer()
		clientConn, serverConn = net.Pipe()
		out                    = json.NewEncoder(clientConn)
		in                     = json.NewDecoder(clientConn)
		successes              = make(chan jsonSuccessResponse)
		failures               = make(chan jsonErrResponse)
		notifications          = make(chan jsonNotification)
		errors                 = make(chan error, 10)
	)

	// setup and start server
	for _, namespace := range namespaces {
		if err := server.RegisterName(namespace, &service); err != nil {
			t.Fatalf("unable to register test service %v", err)
		}
	}

	go server.ServeCodec(NewJSONCodec(serverConn), OptionMethodInvocation|OptionSubscriptions)
	defer server.Stop()

	// wait for message and write them to the given channels
	go waitForMessages(t, in, successes, failures, notifications, errors)

	// create subscriptions one by one
	for i, namespace := range namespaces {
		request := map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", notificationCount, i},
		}

		if err := out.Encode(&request); err != nil {
			t.Fatalf("Could not create subscription: %v", err)
		}
	}

	// create all subscriptions in 1 batch
	var requests []interface{}
	for i, namespace := range namespaces {
		requests = append(requests, map[string]interface{}{
			"id":      i,
			"method":  fmt.Sprintf("%s_subscribe", namespace),
			"version": "2.0",
			"params":  []interface{}{"someSubscription", notificationCount, i},
		})
	}

	if err := out.Encode(&requests); err != nil {
		t.Fatalf("Could not create subscription in batch form: %v", err)
	}

	timeout := time.After(30 * time.Second)
	subids := make(map[string]string, subCount)
	count := make(map[string]int, subCount)
	allReceived := func() bool {
		done := len(count) == subCount
		for _, c := range count {
			if c < notificationCount {
				done = false
			}
		}
		return done
	}

	for !allReceived() {
		select {
		case suc := <-successes: // subscription created
			subids[namespaces[int(suc.Id.(float64))]] = suc.Result.(string)
		case notification := <-notifications:
			count[notification.Params.Subscription]++
		case err := <-errors:
			t.Fatal(err)
		case failure := <-failures:
			t.Errorf("received error: %v", failure.Error)
		case <-timeout:
			for _, namespace := range namespaces {
				subid, found := subids[namespace]
				if !found {
					t.Errorf("subscription for %q not created", namespace)
					continue
				}
				if count, found := count[subid]; !found || count < notificationCount {
					t.Errorf("didn't receive all notifications (%d<%d) in time for namespace %q", count, notificationCount, namespace)
				}
			}
			t.Fatal("timed out")
		}
	}
}

*/

func TestNewID(t *testing.T) {
	hexchars := "0123456789ABCDEFabcdef"
	for i := 0; i < 100; i++ {
		id := string(NewID())
		if !strings.HasPrefix(id, "0x") {
			t.Fatalf("invalid ID prefix, want '0x...', got %s", id)
		}

		id = id[2:]
		if len(id) == 0 || len(id) > 32 {
			t.Fatalf("invalid ID length, want len(id) > 0 && len(id) <= 32), got %d", len(id))
		}

		for i := 0; i < len(id); i++ {
			if strings.IndexByte(hexchars, id[i]) == -1 {
				t.Fatalf("unexpected byte, want any valid hex char, got %c", id[i])
			}
		}
	}
}
