// Copyright 2020 420integrated
// This file is part of the go-420coin library.
//
// The go-420coin library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-420coin library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-420coin library. If not, see <http://www.gnu.org/licenses/>.

package fourtwenty

import (
	"errors"
	"math/big"
	"sync"
	"time"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/420/protocols/420"
	"github.com/420integrated/go-420coin/420/protocols/snap"
	"github.com/420integrated/go-420coin/event"
	"github.com/420integrated/go-420coin/p2p"
)

var (
	// errPeerSetClosed is returned if a peer is attempted to be added or removed
	// from the peer set after it has been terminated.
	errPeerSetClosed = errors.New("peerset closed")

	// errPeerAlreadyRegistered is returned if a peer is attempted to be added
	// to the peer set, but one with the same id already exists.
	errPeerAlreadyRegistered = errors.New("peer already registered")

	// errPeerNotRegistered is returned if a peer is attempted to be removed from
	// a peer set, but no peer with the given id exists.
	errPeerNotRegistered = errors.New("peer not registered")

	// fourtwentyConnectTimeout is the `snap` timeout for `fourtwenty` to connect too.
	fourtwentyConnectTimeout = 3 * time.Second
)

// peerSet represents the collection of active peers currently participating in
// the `fourtwenty` or `snap` protocols.
type peerSet struct {
	fourtwentyPeers  map[string]*fourtwentyPeer  // Peers connected on the `fourtwenty` protocol
	snapPeers map[string]*snapPeer // Peers connected on the `snap` protocol

	fourtwentyJoinFeed  event.Feed // Events when an `fourtwenty` peer successfully joins
	fourtwentyDropFeed  event.Feed // Events when an `fourtwenty` peer gets dropped
	snapJoinFeed event.Feed // Events when a `snap` peer joins on both `fourtwenty` and `snap`
	snapDropFeed event.Feed // Events when a `snap` peer gets dropped (only if fully joined)

	scope event.SubscriptionScope // Subscription group to unsubscribe everyone at once

	lock   sync.RWMutex
	closed bool
}

// newPeerSet creates a new peer set to track the active participants.
func newPeerSet() *peerSet {
	return &peerSet{
		fourtwentyPeers:  make(map[string]*fourtwentyPeer),
		snapPeers: make(map[string]*snapPeer),
	}
}

// subscribeFourtwentyJoin registers a subscription for peers joining (and completing
// the handshake) on the `fourtwenty` protocol.
func (ps *peerSet) subscribeFourtwentyJoin(ch chan<- *fourtwenty.Peer) event.Subscription {
	return ps.scope.Track(ps.fourtwentyJoinFeed.Subscribe(ch))
}

// subscribeFourtwentyDrop registers a subscription for peers being dropped from the
// `fourtwenty` protocol.
func (ps *peerSet) subscribeFourtwentyDrop(ch chan<- *fourtwenty.Peer) event.Subscription {
	return ps.scope.Track(ps.fourtwentyDropFeed.Subscribe(ch))
}

// subscribeSnapJoin registers a subscription for peers joining (and completing
// the `fourtwenty` join) on the `snap` protocol.
func (ps *peerSet) subscribeSnapJoin(ch chan<- *snap.Peer) event.Subscription {
	return ps.scope.Track(ps.snapJoinFeed.Subscribe(ch))
}

// subscribeSnapDrop registers a subscription for peers being dropped from the
// `snap` protocol.
func (ps *peerSet) subscribeSnapDrop(ch chan<- *snap.Peer) event.Subscription {
	return ps.scope.Track(ps.snapDropFeed.Subscribe(ch))
}

// registerFourtwentyPeer injects a new `fourtwenty` peer into the working set, or returns an
// error if the peer is already known. The peer is announced on the `fourtwenty` join
// feed and if it completes a pending `snap` peer, also on that feed.
func (ps *peerSet) registerFourtwentyPeer(peer *fourtwenty.Peer) error {
	ps.lock.Lock()
	if ps.closed {
		ps.lock.Unlock()
		return errPeerSetClosed
	}
	id := peer.ID()
	if _, ok := ps.fourtwentyPeers[id]; ok {
		ps.lock.Unlock()
		return errPeerAlreadyRegistered
	}
	ps.fourtwentyPeers[id] = &fourtwentyPeer{Peer: peer}

	snap, ok := ps.snapPeers[id]
	ps.lock.Unlock()

	if ok {
		// Previously dangling `snap` peer, stop it's timer since `fourtwenty` connected
		snap.lock.Lock()
		if snap.fourtwentyDrop != nil {
			snap.fourtwentyDrop.Stop()
			snap.fourtwentyDrop = nil
		}
		snap.lock.Unlock()
	}
	ps.fourtwentyJoinFeed.Send(peer)
	if ok {
		ps.snapJoinFeed.Send(snap.Peer)
	}
	return nil
}

// unregisterFourtwentyPeer removes a remote peer from the active set, disabling any further
// actions to/from that particular entity. The drop is announced on the `fourtwenty` drop
// feed and also on the `snap` feed if the eth/snap duality was broken just now.
func (ps *peerSet) unregisterFourtwentyPeer(id string) error {
	ps.lock.Lock()
	eth, ok := ps.fourtwentyPeers[id]
	if !ok {
		ps.lock.Unlock()
		return errPeerNotRegistered
	}
	delete(ps.fourtwentyPeers, id)

	snap, ok := ps.snapPeers[id]
	ps.lock.Unlock()

	ps.fourtwentyDropFeed.Send(eth)
	if ok {
		ps.snapDropFeed.Send(snap)
	}
	return nil
}

// registerSnapPeer injects a new `snap` peer into the working set, or returns
// an error if the peer is already known. The peer is announced on the `snap`
// join feed if it completes an existing `fourtwenty` peer.
//
// If the peer isn't yet connected on `fourtwenty` and fails to do so within a given
// amount of time, it is dropped. This enforces that `snap` is an extension to
// `fourtwenty`, not a standalone leeching protocol.
func (ps *peerSet) registerSnapPeer(peer *snap.Peer) error {
	ps.lock.Lock()
	if ps.closed {
		ps.lock.Unlock()
		return errPeerSetClosed
	}
	id := peer.ID()
	if _, ok := ps.snapPeers[id]; ok {
		ps.lock.Unlock()
		return errPeerAlreadyRegistered
	}
	ps.snapPeers[id] = &snapPeer{Peer: peer}

	_, ok := ps.fourtwentyPeers[id]
	if !ok {
		// Dangling `snap` peer, start a timer to drop if `fourtwenty` doesn't connect
		ps.snapPeers[id].fourtwentyDrop = time.AfterFunc(fourtwentyConnectTimeout, func() {
			peer.Log().Warn("Snapshot peer missing fourtwenty, dropping", "addr", peer.RemoteAddr(), "type", peer.Name())
			peer.Disconnect(p2p.DiscUselessPeer)
		})
	}
	ps.lock.Unlock()

	if ok {
		ps.snapJoinFeed.Send(peer)
	}
	return nil
}

// unregisterSnapPeer removes a remote peer from the active set, disabling any
// further actions to/from that particular entity. The drop is announced on the
// `snap` drop feed.
func (ps *peerSet) unregisterSnapPeer(id string) error {
	ps.lock.Lock()
	peer, ok := ps.snapPeers[id]
	if !ok {
		ps.lock.Unlock()
		return errPeerNotRegistered
	}
	delete(ps.snapPeers, id)
	ps.lock.Unlock()

	peer.lock.Lock()
	if peer.fourtwentyDrop != nil {
		peer.fourtwentyDrop.Stop()
		peer.fourtwentyDrop = nil
	}
	peer.lock.Unlock()

	ps.snapDropFeed.Send(peer)
	return nil
}

// fourtwentyPeer retrieves the registered `fourtwenty` peer with the given id.
func (ps *peerSet) fourtwentyPeer(id string) *fourtwentyPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.fourtwentyPeers[id]
}

// snapPeer retrieves the registered `snap` peer with the given id.
func (ps *peerSet) snapPeer(id string) *snapPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.snapPeers[id]
}

// fourtwentyPeersWithoutBlock retrieves a list of `fourtwenty` peers that do not have a given
// block in their set of known hashes so it might be propagated to them.
func (ps *peerSet) fourtwentyPeersWithoutBlock(hash common.Hash) []*fourtwentyPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*fourtwentyPeer, 0, len(ps.fourtwentyPeers))
	for _, p := range ps.fourtwentyPeers {
		if !p.KnownBlock(hash) {
			list = append(list, p)
		}
	}
	return list
}

// fourtwentyPeersWithoutTransacion retrieves a list of `fourtwenty` peers that do not have a
// given transaction in their set of known hashes.
func (ps *peerSet) fourtwentyPeersWithoutTransaction(hash common.Hash) []*fourtwentyPeer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*fourtwentyPeer, 0, len(ps.fourtwentyPeers))
	for _, p := range ps.fourtwentyPeers {
		if !p.KnownTransaction(hash) {
			list = append(list, p)
		}
	}
	return list
}

// Len returns if the current number of `fourtwenty` peers in the set. Since the `snap`
// peers are tied to the existnce of an `fourtwenty` connection, that will always be a
// subset of `fourtwenty`.
func (ps *peerSet) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.fourtwentyPeers)
}

// fourtwentyPeerWithHighestTD retrieves the known peer with the currently highest total
// difficulty.
func (ps *peerSet) fourtwentyPeerWithHighestTD() *fourtwenty.Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	var (
		bestPeer *fourtwenty.Peer
		bestTd   *big.Int
	)
	for _, p := range ps.fourtwentyPeers {
		if _, td := p.Head(); bestPeer == nil || td.Cmp(bestTd) > 0 {
			bestPeer, bestTd = p.Peer, td
		}
	}
	return bestPeer
}

// close disconnects all peers.
func (ps *peerSet) close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.fourtwentyPeers {
		p.Disconnect(p2p.DiscQuitting)
	}
	for _, p := range ps.snapPeers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}
