// This test calls the test_echo method.

SEND {"jsonrpc": "2.0", "id": 2, "method": "test_echo", "params": []}
RECV {"jsonrpc":"2.0","id":2,"error":{"code":-32602,"message":"missing value for required argument 0"}}

SEND {"jsonrpc": "2.0", "id": 2, "method": "test_echo", "params": ["x"]}
RECV {"jsonrpc":"2.0","id":2,"error":{"code":-32602,"message":"missing value for required argument 1"}}

SEND {"jsonrpc": "2.0", "id": 2, "method": "test_echo", "params": ["x", 3]}
RECV {"jsonrpc":"2.0","id":2,"result":{"String":"x","Int":3,"Args":null}}

SEND {"jsonrpc": "2.0", "id": 2, "method": "test_echo", "params": ["x", 3, {"S": "foo"}]}
RECV {"jsonrpc":"2.0","id":2,"result":{"String":"x","Int":3,"Args":{"S":"foo"}}}

SEND {"jsonrpc": "2.0", "id": 2, "method": "test_echoWithCtx", "params": ["x", 3, {"S": "foo"}]}
RECV {"jsonrpc":"2.0","id":2,"result":{"String":"x","Int":3,"Args":{"S":"foo"}}}
