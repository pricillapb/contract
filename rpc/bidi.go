package rpc

import "context"

// +build none

/*

Planned interfaces for bidirectional RPC
----------------------------------------

Server and client use 'handler' object underneath. The handler implements
message dispatch for requests, responses, and notifications.

There is still a big difference between clients and servers.

*/

type Server struct{}

// RegisterName adds a method call receiver to the server.
func (Server) RegisterName(namespace string, api interface{}) {}

type Client struct{}

func (Client) Notify(method string, params ...interface{}) error {}

func (Client) Call(result interface{}, method string, params ...interface{}) error {}

func (Client) CallContext(ctx context.Context, result interface{}, method string, args ...interface{}) error {
}

type API struct{}

func (API) MyMethod(ctx context.Context, param interface{}) error {
	// Get notifier to talk back to client.
	n, ok := NotifierFromContext(ctx)
	if !ok {
		// bidi communication not supported
	}

	// Sending notifications:
	param := 3
	n.Notify("method", param)

	// Making calls:
	var result int
	n.Call(&result, "method", param)
}
