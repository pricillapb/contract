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

package les

import (
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/eth/downloader"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/logger"
	"github.com/ethereum/go-ethereum/logger/glog"
	"github.com/ethereum/go-ethereum/p2p/discover"
	"golang.org/x/net/context"
)

const (
	//forceSyncCycle      = 10 * time.Second // Time interval to force syncs, even if few peers are available
	idealPeerCount = 2 // Amount of peers desired to start syncing
)

// syncer is responsible for periodically synchronising with the network, both
// downloading hashes and blocks as well as handling the announcement handler.
func (pm *ProtocolManager) syncer() {
	defer pm.downloader.Terminate()

	var (
		searchStop   chan struct{}
		searchResult = make(chan string, 100)
		added        = make(map[discover.NodeID]bool)
	)
	for {
		if pm.peers.Len() >= idealPeerCount {
			if searchStop != nil {
				close(searchStop)
				searchStop = nil
			}
		} else {
			if searchStop == nil && pm.topicDisc != nil {
				searchStop = make(chan struct{})
				go pm.topicDisc.SearchTopic(pm.lesTopic, searchStop, searchResult)
			}
		}

		select {
		case enode := <-searchResult:
			node, err := discover.ParseNode(enode)
			if err != nil || added[node.ID] {
				continue
			}
			glog.V(logger.Info).Infoln("Found LES server:", enode)
			pm.p2pServer.AddPeer(node)
		case <-pm.newPeerCh:
			// continue looking
		case <-pm.noMorePeers:
			return
		}
	}
}

func (pm *ProtocolManager) needToSync(peerHead blockInfo) bool {
	head := pm.blockchain.CurrentHeader()
	currentTd := core.GetTd(pm.chainDb, head.Hash(), head.Number.Uint64())
	return currentTd != nil && peerHead.Td.Cmp(currentTd) > 0
}

// synchronise tries to sync up our local block chain with a remote peer.
func (pm *ProtocolManager) synchronise(peer *peer) {
	// Short circuit if no peers are available
	if peer == nil {
		return
	}

	// Make sure the peer's TD is higher than our own.
	if !pm.needToSync(peer.headBlockInfo()) {
		return
	}

	ctx, _ := context.WithTimeout(context.Background(), time.Second*5)
	pm.blockchain.(*light.LightChain).SyncCht(ctx)

	pm.downloader.Synchronise(peer.id, peer.Head(), peer.Td(), downloader.LightSync)
}
