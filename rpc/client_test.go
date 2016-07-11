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
	"net"
	"reflect"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/net/context"
)

func newTestClient(t *testing.T, serviceName string, service interface{}) (*Server, *Client) {
	server := NewServer()
	if err := server.RegisterName(serviceName, service); err != nil {
		t.Fatal(err)
	}
	return server, DialInProc(server)
}

func TestClientRequest(t *testing.T) {
	server, client := newTestClient(t, "service", new(Service))
	defer server.Stop()
	defer client.Close()

	var resp Result
	if err := client.Call(&resp, "service_echo", "hello", 10, &Args{"world"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(resp, Result{"hello", 10, &Args{"world"}}) {
		t.Errorf("incorrect result %#v", resp)
	}
}

func TestClientBatchRequest(t *testing.T) {
	server, client := newTestClient(t, "service", new(Service))
	defer server.Stop()
	defer client.Close()

	batch := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: new(Result),
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
		},
	}
	if err := client.BatchCall(batch); err != nil {
		t.Fatal(err)
	}
	wantResult := []BatchElem{
		{
			Method: "service_echo",
			Args:   []interface{}{"hello", 10, &Args{"world"}},
			Result: &Result{"hello", 10, &Args{"world"}},
		},
		{
			Method: "service_echo",
			Args:   []interface{}{"hello2", 11, &Args{"world"}},
			Result: &Result{"hello2", 11, &Args{"world"}},
		},
		{
			Method: "no_such_method",
			Args:   []interface{}{1, 2, 3},
			Result: new(int),
			Error:  &JSONError{Code: -32601, Message: "The method no_such_method_ does not exist/is not available"},
		},
	}
	if !reflect.DeepEqual(batch, wantResult) {
		t.Errorf("batch results mismatch:\ngot %swant %s", spew.Sdump(batch), spew.Sdump(wantResult))
	}
}

func TestClientSubscribeInvalidArg(t *testing.T) {
	check := func(shouldPanic bool, arg interface{}) {
		defer func() {
			err := recover()
			if shouldPanic && err == nil {
				t.Errorf("EthSubscribe should've panicked for %#v", arg)
			}
			if !shouldPanic && err != nil {
				t.Errorf("EthSubscribe shouldn't have panicked for %#v", arg)
				buf := make([]byte, 1024*1024)
				buf = buf[:runtime.Stack(buf, false)]
				t.Error(err)
				t.Error(string(buf))
			}
		}()
		server, client := newTestClient(t, "service", new(Service))
		defer server.Stop()
		defer client.Close()
		client.EthSubscribe(arg, "foo_bar")
	}
	check(true, nil)
	check(true, 1)
	check(true, (chan int)(nil))
	check(true, make(<-chan int))
	check(false, make(chan int))
	check(false, make(chan<- int))
}

func TestClientSubscribe(t *testing.T) {
	server, client := newTestClient(t, "eth", new(NotificationTestService))
	defer server.Stop()
	defer client.Close()

	nc := make(chan int)
	count := 10
	sub, err := client.EthSubscribe(nc, "someSubscription", count, 0)
	if err != nil {
		t.Fatal("can't subscribe:", err)
	}
	for i := 0; i < count; i++ {
		if val := <-nc; val != i {
			t.Fatalf("value mismatch: got %, want %d", val, i)
		}
	}

	sub.Unsubscribe()
	select {
	case _, ok := <-nc:
		if ok {
			t.Fatal("channel was not closed after unsubscribe")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("channel not closed within 1s after unsubscribe")
	}
	if err := sub.Err(); err != nil {
		t.Fatalf("Err returned a non-nil error after explicit unsubscribe: %q", err)
	}
}

// In this test, the connection drops while EthSubscribe is
// waiting for a response.
func TestClientSubscribeClose(t *testing.T) {
	service := &NotificationTestService{
		gotHangSubscriptionReq:  make(chan struct{}),
		unblockHangSubscription: make(chan struct{}),
	}
	server, client := newTestClient(t, "eth", service)
	defer server.Stop()
	defer client.Close()

	var (
		nc   = make(chan int)
		errc = make(chan error)
		sub  *ClientSubscription
		err  error
	)
	go func() {
		sub, err = client.EthSubscribe(nc, "hangSubscription", 999)
		errc <- err
	}()

	<-service.gotHangSubscriptionReq
	client.Close()
	service.unblockHangSubscription <- struct{}{}

	select {
	case err := <-errc:
		if err == nil {
			t.Errorf("EthSubscribe returned nil error after Close")
		}
		if sub != nil {
			t.Error("EthSubscribe returned non-nil subscription after Close")
		}
	case <-time.After(1 * time.Second):
		t.Fatalf("EthSubscribe did not return within 1s after Close")
	}
}

// TODO: test subscribing the same channel multiple times

func TestClientHTTP(t *testing.T) {
	server := NewServer()
	server.RegisterName("service", new(Service))
	httpserver := NewHTTPServer("", server)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go httpserver.Serve(listener)

	// Launch concurrent requests.
	var (
		client, _  = Dial("http://" + listener.Addr().String())
		results    = make([]Result, 100)
		errc       = make(chan error)
		wantResult = Result{"a", 1, new(Args)}
	)
	defer client.Close()
	for i := range results {
		i := i
		go func() {
			errc <- client.Call(&results[i], "service_echo",
				wantResult.String, wantResult.Int, wantResult.Args)
		}()
	}

	// Wait for all of them to complete.
	timeout := time.NewTimer(5 * time.Second)
	defer timeout.Stop()
	for i := range results {
		select {
		case err := <-errc:
			if err != nil {
				t.Fatal(err)
			}
		case <-timeout.C:
			t.Fatalf("timeout (got %d/%d) results)", i+1, len(results))
		}
	}

	// Check results.
	for i := range results {
		if !reflect.DeepEqual(results[i], wantResult) {
			t.Errorf("result %d mismatch: got %#v, want %#v", results[i], wantResult)
		}
	}
}

func TestClientReconnect(t *testing.T) {
	startServer := func(addr string) (*Server, net.Listener) {
		server := NewServer()
		server.RegisterName("service", new(Service))
		l, err := ListenWS(server, addr, "*")
		if err != nil {
			t.Fatal("can't serve", err)
		}
		return server, l
	}

	// Start a server and corresponding client.
	s1, l1 := startServer("127.0.0.1:0")
	client, err := Dial("ws://" + l1.Addr().String())
	if err != nil {
		t.Fatal("can't dial", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform a call. This should work because the server is up.
	var resp Result
	if err := client.CallContext(ctx, &resp, "service_echo", "", 1, nil); err != nil {
		t.Fatal(err)
	}

	// Shut down the server and try calling again. It shouldn't work.
	l1.Close()
	s1.Stop()
	if err := client.CallContext(ctx, &resp, "service_echo", "", 2, nil); err == nil {
		t.Error("successful call while the server is down")
		t.Logf("resp: %#v", resp)
	}

	// Allow for some cool down time so we can listen on the same address again.
	time.Sleep(2 * time.Second)

	// Start it up again and call again. The connection should be reestablished.
	// We spawn multiple calls here to check whether this hangs somehow.
	s2, l2 := startServer(l1.Addr().String())
	defer l2.Close()
	defer s2.Stop()

	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			var resp Result
			if err := client.CallContext(ctx, &resp, "service_echo", "", 3, nil); err != nil {
				t.Error("call with reconnect failed:", err)
			}
		}()
	}
	close(start)
	wg.Wait()
}
