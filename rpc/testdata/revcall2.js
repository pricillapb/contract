// This test checks reverse calls.

--> {"jsonrpc":"2.0","id":2,"method":"test_callMeBackLater","params":["foo",[1]]}
<-- {"jsonrpc":"2.0","id":2,"result":null}
<-- {"jsonrpc":"2.0","id":"0x1","method":"foo","params":[1]}
--> {"jsonrpc":"2.0","id":"0x1","result":"my result"}

