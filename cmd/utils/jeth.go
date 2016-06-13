// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

package utils

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/jsre"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/robertkrimen/otto"
)

type Jeth struct {
	re     *jsre.JSRE
	client *rpc.Client
}

// NewJeth create a new backend for the JSRE console
func NewJeth(re *jsre.JSRE, client *rpc.Client) *Jeth {
	return &Jeth{re, client}
}

// err returns an error object for the given error code and message.
func (self *Jeth) err(call otto.FunctionCall, code int, msg string, id interface{}) (response otto.Value) {
	m := rpc.JSONErrResponse{
		Version: "2.0",
		Id:      id,
		Error: rpc.JSONError{
			Code:    code,
			Message: msg,
		},
	}

	errObj, _ := json.Marshal(m.Error)
	errRes, _ := json.Marshal(m)

	call.Otto.Run("ret_error = " + string(errObj))
	res, _ := call.Otto.Run("ret_response = " + string(errRes))

	return res
}

// UnlockAccount asks the user for the password and than executes the jeth.UnlockAccount callback in the jsre.
// It will need the public address for the account to unlock as first argument.
// The second argument is an optional string with the password. If not given the user is prompted for the password.
// The third argument is an optional integer which specifies for how long the account will be unlocked (in seconds).
func (self *Jeth) UnlockAccount(call otto.FunctionCall) (response otto.Value) {
	var account, passwd otto.Value
	duration := otto.NullValue()

	if !call.Argument(0).IsString() {
		fmt.Println("first argument must be the account to unlock")
		return otto.FalseValue()
	}

	account = call.Argument(0)

	// if password is not given or as null value -> ask user for password
	if call.Argument(1).IsUndefined() || call.Argument(1).IsNull() {
		fmt.Printf("Unlock account %s\n", account)
		if input, err := Stdin.PasswordPrompt("Passphrase: "); err != nil {
			throwJSExeception(err.Error())
		} else {
			passwd, _ = otto.ToValue(input)
		}
	} else {
		if !call.Argument(1).IsString() {
			throwJSExeception("password must be a string")
		}
		passwd = call.Argument(1)
	}

	// third argument is the duration how long the account must be unlocked.
	// verify that its a number.
	if call.Argument(2).IsDefined() && !call.Argument(2).IsNull() {
		if !call.Argument(2).IsNumber() {
			throwJSExeception("unlock duration must be a number")
		}
		duration = call.Argument(2)
	}

	// jeth.unlockAccount will send the request to the backend.
	if val, err := call.Otto.Call("jeth.unlockAccount", nil, account, passwd, duration); err == nil {
		return val
	} else {
		throwJSExeception(err.Error())
	}

	return otto.FalseValue()
}

// NewAccount asks the user for the password and than executes the jeth.newAccount callback in the jsre
func (self *Jeth) NewAccount(call otto.FunctionCall) (response otto.Value) {
	var passwd string
	if len(call.ArgumentList) == 0 {
		var err error
		passwd, err = Stdin.PasswordPrompt("Passphrase: ")
		if err != nil {
			return otto.FalseValue()
		}
		passwd2, err := Stdin.PasswordPrompt("Repeat passphrase: ")
		if err != nil {
			return otto.FalseValue()
		}

		if passwd != passwd2 {
			fmt.Println("Passphrases don't match")
			return otto.FalseValue()
		}
	} else if len(call.ArgumentList) == 1 && call.Argument(0).IsString() {
		passwd, _ = call.Argument(0).ToString()
	} else {
		fmt.Println("expected 0 or 1 string argument")
		return otto.FalseValue()
	}

	ret, err := call.Otto.Call("jeth.newAccount", nil, passwd)
	if err == nil {
		return ret
	}
	fmt.Println(err)
	return otto.FalseValue()
}

type jsonrpcCall struct {
	Id     int64
	Method string
	Params []interface{}
}

// Send implements the web3 provider "send" method.
func (self *Jeth) Send(call otto.FunctionCall) (response otto.Value) {
	// Remarshal the request into a Go value.
	JSON, _ := call.Otto.Object("JSON")
	reqVal, err := JSON.Call("stringify", call.Argument(0))
	if err != nil {
		throwJSExeception(err.Error())
	}
	var (
		rawReq = []byte(reqVal.String())
		reqs   []jsonrpcCall
		batch  bool
	)
	if rawReq[0] == '[' {
		batch = true
		json.Unmarshal(rawReq, &reqs)
	} else {
		batch = false
		reqs = make([]jsonrpcCall, 1)
		json.Unmarshal(rawReq, &reqs[0])
	}

	// Execute the requests.
	resps, _ := call.Otto.Object("new Array()")
	for _, req := range reqs {
		resp, _ := call.Otto.Object(`({"jsonrpc":"2.0"})`)
		resp.Set("id", req.Id)
		var result interface{}
		err = self.client.Call(&result, req.Method, req.Params...)
		switch err := err.(type) {
		case nil:
			resp.Set("result", result)
		case *rpc.JSONError:
			resp.Set("error", map[string]interface{}{
				"code":    err.Code,
				"message": err.Message,
			})
		default:
			resp = self.err(call, -32603, err.Error(), &req.Id).Object()
		}
		resps.Call("push", resp)
	}

	// Return the responses either to the callback (if supplied)
	// or directly as the return value.
	if batch {
		response = resps.Value()
	} else {
		response, _ = resps.Get("0")
	}
	if fn := call.Argument(1).Object(); fn != nil && fn.Class() == "function" {
		fn.Call("apply", response)
		return otto.UndefinedValue()
	}
	return response
}

// throwJSExeception panics on an otto value, the Otto VM will then throw msg as a javascript error.
func throwJSExeception(msg interface{}) otto.Value {
	p, _ := otto.ToValue(msg)
	panic(p)
}

// Sleep will halt the console for arg[0] seconds.
func (self *Jeth) Sleep(call otto.FunctionCall) (response otto.Value) {
	if len(call.ArgumentList) >= 1 {
		if call.Argument(0).IsNumber() {
			sleep, _ := call.Argument(0).ToInteger()
			time.Sleep(time.Duration(sleep) * time.Second)
			return otto.TrueValue()
		}
	}
	return throwJSExeception("usage: sleep(<sleep in seconds>)")
}

// SleepBlocks will wait for a specified number of new blocks or max for a
// given of seconds. sleepBlocks(nBlocks[, maxSleep]).
func (self *Jeth) SleepBlocks(call otto.FunctionCall) (response otto.Value) {
	nBlocks := int64(0)
	maxSleep := int64(9999999999999999) // indefinitely

	nArgs := len(call.ArgumentList)

	if nArgs == 0 {
		throwJSExeception("usage: sleepBlocks(<n blocks>[, max sleep in seconds])")
	}

	if nArgs >= 1 {
		if call.Argument(0).IsNumber() {
			nBlocks, _ = call.Argument(0).ToInteger()
		} else {
			throwJSExeception("expected number as first argument")
		}
	}

	if nArgs >= 2 {
		if call.Argument(1).IsNumber() {
			maxSleep, _ = call.Argument(1).ToInteger()
		} else {
			throwJSExeception("expected number as second argument")
		}
	}

	// go through the console, this will allow web3 to call the appropriate
	// callbacks if a delayed response or notification is received.
	currentBlockNr := func() int64 {
		result, err := call.Otto.Run("eth.blockNumber")
		if err != nil {
			throwJSExeception(err.Error())
		}
		blockNr, err := result.ToInteger()
		if err != nil {
			throwJSExeception(err.Error())
		}
		return blockNr
	}

	targetBlockNr := currentBlockNr() + nBlocks
	deadline := time.Now().Add(time.Duration(maxSleep) * time.Second)

	for time.Now().Before(deadline) {
		if currentBlockNr() >= targetBlockNr {
			return otto.TrueValue()
		}
		time.Sleep(time.Second)
	}

	return otto.FalseValue()
}
