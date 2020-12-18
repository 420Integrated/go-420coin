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
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/420/protocols/snap"
	"github.com/420integrated/go-420coin/p2p/enode"
)

// snapHandler implements the snap.Backend interface to handle the various network
// packets that are sent as replies or broadcasts.
type snapHandler handler

func (h *snapHandler) Chain() *core.BlockChain { return h.chain }

// RunPeer is invoked when a peer joins on the `snap` protocol.
func (h *snapHandler) RunPeer(peer *snap.Peer, hand snap.Handler) error {
	return (*handler)(h).runSnapPeer(peer, hand)
}

// PeerInfo retrieves all known `snap` information about a peer.
func (h *snapHandler) PeerInfo(id enode.ID) interface{} {
	if p := h.peers.snapPeer(id.String()); p != nil {
		return p.info()
	}
	return nil
}

// Handle is invoked from a peer's message handler when it receives a new remote
// message that the handler couldn't consume and serve itself.
func (h *snapHandler) Handle(peer *snap.Peer, packet snap.Packet) error {
	return h.downloader.DeliverSnapPacket(peer, packet)
}