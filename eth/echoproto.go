package eth

import (
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/ethereum/go-ethereum/p2p"
)

var echoProtocol = p2p.Protocol{
	Length:  2,
	Name:    "echo",
	Version: 1,
	Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
		return echoLoop(rw)
	},
}

func echoLoop(rw p2p.MsgReadWriter) error {
	for {
		msg, err := rw.ReadMsg()
		if err != nil {
			return fmt.Errorf("read error: %v", err)
		}
		fmt.Printf("got message (code %d, size %d)\n", msg.Code, msg.Size)
		time.Sleep(1 * time.Second)
		if msg.Code == 0 {
			// echo request, stream input data back
			err = rw.WriteMsg(p2p.Msg{Code: 1, Size: msg.Size, Payload: msg.Payload})
		} else {
			_, err = io.Copy(ioutil.Discard, msg.Payload)
		}
		if err != nil {
			return err
		}
	}
}
