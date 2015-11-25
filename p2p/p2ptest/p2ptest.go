package p2ptest

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"

	"github.com/ethereum/go-ethereum/p2p"
	"github.com/ethereum/go-ethereum/p2p/discover"
)

type topologyAlg int

const (
	Ring topologyAlg = iota
)

type Network struct {
	N        int          // how many nodes
	Topology topologyAlg  // connected in which way
	Proto    p2p.Protocol // launched by Start

	peers []*p2p.Peer
	pipes []*MsgPipeRW
	errc  chan error
	wg    sync.WaitGroup
}

func (net *Network) Start() {
	cap := []p2p.Cap{{Name: net.Proto.Name, Version: net.Proto.Version}}
	net.peers = make([]*p2p.Peer, net.N)
	for i := range net.peers {
		var id discover.NodeID
		binary.BigEndian.PutUint32(id[len(id)-8:], uint32(i))
		net.peers[i] = p2p.NewPeer(id, fmt.Sprintf("test %d", i), cap)
	}
	net.errc = make(chan error, net.N*2)
	switch net.Topology {
	case Ring:
		net.ringTopology()
	default:
		panic(fmt.Errorf("unknown topology %d", net.Topology))
	}
}

func (net *Network) Stop() {
	for _, p := range net.pipes {
		p.Close()
	}
	net.wg.Wait()
}

func (net *Network) ringTopology() {
	for i := 1; i < len(net.peers); i++ {
		net.launch(net.peers[i], net.peers[i-1])
	}
}

func (net *Network) launch(peer1, peer2 *p2p.Peer) {
	rw1, rw2 := MsgPipe()
	net.pipes = append(net.pipes, rw1)
	net.wg.Add(2)
	go func() { net.errc <- net.Proto.Run(peer1, rw1) }()
	go func() { net.errc <- net.Proto.Run(peer2, rw2) }()
}

// MsgPipe creates a message pipe. Reads on one end are matched
// with writes on the other. The pipe is full-duplex, both ends
// implement MsgReadWriter.
func MsgPipe() (*MsgPipeRW, *MsgPipeRW) {
	var (
		c1, c2  = make(chan p2p.Msg), make(chan p2p.Msg)
		closing = make(chan struct{})
		closed  = new(int32)
		rw1     = &MsgPipeRW{c1, c2, closing, closed}
		rw2     = &MsgPipeRW{c2, c1, closing, closed}
	)
	return rw1, rw2
}

// ErrPipeClosed is returned from pipe operations after the
// pipe has been closed.
var ErrPipeClosed = errors.New("p2p: read or write on closed message pipe")

// MsgPipeRW is an endpoint of a MsgReadWriter pipe.
type MsgPipeRW struct {
	w       chan<- p2p.Msg
	r       <-chan p2p.Msg
	closing chan struct{}
	closed  *int32
}

// WriteMsg sends a messsage on the pipe.
// It blocks until the receiver has consumed the message payload.
func (p *MsgPipeRW) WriteMsg(msg p2p.Msg) error {
	if atomic.LoadInt32(p.closed) == 0 {
		consumed := make(chan struct{}, 1)
		msg.Payload = &eofSignal{msg.Payload, msg.Size, consumed}
		select {
		case p.w <- msg:
			if msg.Size > 0 {
				// wait for payload read or discard
				select {
				case <-consumed:
				case <-p.closing:
				}
			}
			return nil
		case <-p.closing:
		}
	}
	return ErrPipeClosed
}

// ReadMsg returns a message sent on the other end of the pipe.
func (p *MsgPipeRW) ReadMsg() (p2p.Msg, error) {
	if atomic.LoadInt32(p.closed) == 0 {
		select {
		case msg := <-p.r:
			return msg, nil
		case <-p.closing:
		}
	}
	return p2p.Msg{}, ErrPipeClosed
}

// Close unblocks any pending ReadMsg and WriteMsg calls on both ends
// of the pipe. They will return ErrPipeClosed. Close also
// interrupts any reads from a message payload.
func (p *MsgPipeRW) Close() error {
	if atomic.AddInt32(p.closed, 1) != 1 {
		// someone else is already closing
		atomic.StoreInt32(p.closed, 1) // avoid overflow
		return nil
	}
	close(p.closing)
	return nil
}

// eofSignal wraps a reader with eof signaling. the eof channel is
// closed when the wrapped reader returns an error or when count bytes
// have been read.
type eofSignal struct {
	wrapped io.Reader
	count   uint32 // number of bytes left
	eof     chan<- struct{}
}

// note: when using eofSignal to detect whether a message payload
// has been read, Read might not be called for zero sized messages.
func (r *eofSignal) Read(buf []byte) (int, error) {
	if r.count == 0 {
		if r.eof != nil {
			r.eof <- struct{}{}
			r.eof = nil
		}
		return 0, io.EOF
	}

	max := len(buf)
	if int(r.count) < len(buf) {
		max = int(r.count)
	}
	n, err := r.wrapped.Read(buf[:max])
	r.count -= uint32(n)
	if (err != nil || r.count == 0) && r.eof != nil {
		r.eof <- struct{}{} // tell Peer that msg has been consumed
		r.eof = nil
	}
	return n, err
}
