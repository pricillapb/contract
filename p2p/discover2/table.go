package discover

import (
	"net"
	"sort"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	alpha      = 3  // Kademlia concurrency factor
	bucketSize = 16 // Kademlia bucket size
	hashBits   = len(common.Hash{}) * 8
	nBuckets   = hashBits + 1 // Number of buckets

	maxBondingPingPongs = 16
	maxFindnodeFailures = 5

	autoRefreshInterval = 1 * time.Hour
	seedCount           = 30
	seedMaxAge          = 5 * 24 * time.Hour
)

type transport interface {
	send(to *Node, ptype byte, p handler)
}

type handler interface {
	handle(t transport, tab *Table, from *Node) error
}

type packet struct {
	sender     NodeID
	senderAddr *net.UDPAddr
	rpc        handler
}

type Table struct {
	buckets [nBuckets]*bucket // index of known nodes by distance
	nursery []*Node           // bootstrap nodes
	db      *nodeDB           // database of known nodes
	self    *Node             // metadata of the local node
	net     transport

	refreshReq chan []*Node
	rpcReq     chan packet
	callOnLoop chan func()

	close chan struct{}
	wg    *sync.WaitGroup
}

// bucket contains nodes, ordered by their last activity. the entry
// that was most recently active is the first element in entries.
type bucket struct{ entries []*Node }

// nodesByDistance is a list of nodes, ordered by
// distance to target.
type nodesByDistance struct {
	entries []*Node
	target  common.Hash
}

// push adds the given node to the list, keeping the total size below maxElems.
func (h *nodesByDistance) push(n *Node, maxElems int) {
	ix := sort.Search(len(h.entries), func(i int) bool {
		return distcmp(h.target, h.entries[i].sha, n.sha) > 0
	})
	if len(h.entries) < maxElems {
		h.entries = append(h.entries, n)
	}
	if ix == len(h.entries) {
		// farther away than all nodes we already have.
		// if there was room for it, the node is now the last element.
	} else {
		// slide existing entries down to make room
		// this will overwrite the entry we just appended.
		copy(h.entries[ix+1:], h.entries[ix:])
		h.entries[ix] = n
	}
}

func newTable(self *Node, t transport) *Table {
	tab := &Table{
		rpcReq:     make(chan packet),
		refreshReq: make(chan []*Node),
		callOnLoop: make(chan func()),
		close:      make(chan struct{}),
		wg:         new(sync.WaitGroup),
		net:        t,
		self:       self,
	}
	for i := range tab.buckets {
		tab.buckets[i] = new(bucket)
	}
	tab.wg.Add(1)
	go tab.run()
	return tab
}

func (tab *Table) Close() {
	close(tab.close)
	tab.wg.Wait()
}

func (tab *Table) run() {
	defer tab.wg.Done()
	for {
		select {
		case req := <-tab.rpcReq:
			n := tab.getNode(req.sender, true)
			req.rpc.handle(tab.net, tab, n)
		case bn := <-tab.refreshReq:
			if bn != nil {
				tab.nursery = bn
			}
			// tab.doRefresh()
		case fn := <-tab.callOnLoop:
			fn()
		case <-tab.close:
			return
		}
	}
}

func (tab *Table) getNode(id NodeID, bump bool) *Node {
	idsha := crypto.Sha3Hash(id[:])
	spew.Dump("getNode", tab)
	b := tab.buckets[logdist(tab.self.sha, idsha)]
	for i, n := range b.entries {
		if b.entries[i].ID == id {
			if bump {
				// move it to the front
				copy(b.entries[1:], b.entries[:i])
				b.entries[0] = n
			}
			return n
		}
	}
	return tab.db.node(id)
}

// closest returns the n nodes in the table that are closest to the
// given id. The caller must hold tab.mutex.
func (tab *Table) closest(target common.Hash, nresults int) *nodesByDistance {
	// This is a very wasteful way to find the closest nodes but
	// obviously correct. I believe that tree-based buckets would make
	// this easier to implement efficiently.
	close := &nodesByDistance{target: target}
	for _, b := range tab.buckets {
		for _, n := range b.entries {
			close.push(n, nresults)
		}
	}
	return close
}

/*

bonding flow:

triggered by ping:
    unknown --(ping, sendping)-> bonding --(pong)-> known
triggered locally (by bondWith)
    unknown --(send ping)-> bonding --(pong)-> waitPing -(ping)--> known

*/

func (tab *Table) bondWith(n *Node) {
	if n.bondState == knownState {
		return
	}
	tab.net.send(n, pingPacket, ping{})
}

func (req ping) handle(t transport, tab *Table, n *Node) error {
	t.send(n, pongPacket, pong{})

	switch n.bondState {
	case knownState:
		return nil
	case initialBondState:
		t.send(n, pingPacket, ping{})
		n.bondState = waitPongState
	case waitPingState:
		n.bondState = knownState
	}
	return nil
}

func (req pong) handle(t transport, tab *Table, n *Node) error {
	if n.bondState != waitPongState {
		return errUnsolicitedReply
	}
	n.bondState = knownState
	return nil
}

/*

findnode flow:

findnode is only accepted from known nodes.
neighbours packet contents are pinged if they are not known.

*/

func (req findnode) handle(t transport, tab *Table, n *Node) error {
	if n.bondState == knownState {
		return errUnknownNode
	}
	closest := tab.closest(crypto.Sha3Hash(req.Target[:]), 16).entries
	conv := make([]rpcNode, len(closest))
	for i := range closest {
		conv[i] = nodeToRPC(closest[i])
	}
	t.send(n, neighborsPacket, neighbors{Nodes: conv})
	return nil
}

func (req neighbors) handle(t transport, tab *Table, n *Node) error {
	// dispatch to lookups
	return nil
}
