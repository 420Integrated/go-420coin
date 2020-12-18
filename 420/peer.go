// Copyright 2015 The The 420Integrated Development Group
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
	"math/big"
	"sync"
	"time"

	"github.com/420integrated/go-420coin/420/protocols/420"
	"github.com/420integrated/go-420coin/420/protocols/snap"
)

// fourtwentyPeerInfo represents a short summary of the `fourtwenty` sub-protocol metadata known
// about a connected peer.
type fourtwentyPeerInfo struct {
	Version    uint      `json:"version"`    // 420coin protocol version negotiated
	Difficulty *big.Int `json:"difficulty"` // Total difficulty of the peer's blockchain
	Head       string   `json:"head"`       // Hex hash of the peer's best owned block
}

// fourtwentyPeer is a wrapper around fourtwenty.Peer to maintain a few extra metadata.
type fourtwentyPeer struct {
	*fourtwenty.Peer

	syncDrop *time.Timer  // Connection dropper if `fourtwenty` sync progress isn't validated in time
	lock     sync.RWMutex // Mutex protecting the internal fields}
}

// info gathers and returns some `fourtwenty` protocol metadata known about a peer.
func (p *fourtwentyPeer) info() *fourtwentyPeerInfo {
	hash, td := p.Head()

	return &fourtwentyPeerInfo{
		Version:    p.Version(),
		Difficulty: td,
		Head:       hash.Hex(),
	}
}

// snapPeerInfo represents a short summary of the `snap` sub-protocol metadata known
// about a connected peer.
type snapPeerInfo struct {
	Version uint `json:"version"` // Snapshot protocol version negotiated
}

// snapPeer is a wrapper around snap.Peer to maintain a few extra metadata.
type snapPeer struct {
	*snap.Peer

	fourtwentyDrop *time.Timer  // Connection dropper if `fourtwenty` doesn't connect in time
	lock    sync.RWMutex // Mutex protecting the internal fields
}

// info gathers and returns some `snap` protocol metadata known about a peer.
func (p *snapPeer) info() *snapPeerInfo {
	return &snapPeerInfo{
		Version: p.Version(),
	}
}