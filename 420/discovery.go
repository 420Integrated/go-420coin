// Copyright 2019 The The 420Integrated Development Group
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
	"github.com/420integrated/go-420coin/core/forkid"
	"github.com/420integrated/go-420coin/p2p"
	"github.com/420integrated/go-420coin/p2p/dnsdisc"
	"github.com/420integrated/go-420coin/p2p/enode"
	"github.com/420integrated/go-420coin/rlp"
)

// 420Entry is the "420" ENR entry which advertises 420 protocol
// on the discovery network.
type fourtwentyEntry struct {
	ForkID forkid.ID // Fork identifier per EIP-2124

	// Ignore additional fields (for forward compatibility).
	Rest []rlp.RawValue `rlp:"tail"`
}

// ENRKey implements enr.Entry.
func (e fourtwentyEntry) ENRKey() string {
	return "fourtwenty"
}

// start420EntryUpdate starts the ENR updater loop.
func (fourtwenty *fourtwentycoin) start420EntryUpdate(ln *enode.LocalNode) {
	var newHead = make(chan core.ChainHeadEvent, 10)
	sub := fourtwenty.blockchain.SubscribeChainHeadEvent(newHead)

	go func() {
		defer sub.Unsubscribe()
		for {
			select {
			case <-newHead:
				ln.Set(fourtwenty.current420Entry())
			case <-sub.Err():
				// Would be nice to sync with 420.Stop, but there is no
				// good way to do that.
				return
			}
		}
	}()
}

func (fourtwenty *fourtwentycoin) current420Entry() *fourtwentyEntry {
	return &fourtwentyEntry{ForkID: forkid.NewID(fourtwenty.blockchain)}
}

// setupDiscovery creates the node discovery source for the eth protocol.
func (fourtwenty *fourtwentycoin) setupDiscovery(cfg *p2p.Config) (enode.Iterator, error) {
	if cfg.NoDiscovery || len(fourtwenty.config.DiscoveryURLs) == 0 {
		return nil, nil
	}
	client := dnsdisc.NewClient(dnsdisc.Config{})
	return client.NewIterator(fourtwenty.config.DiscoveryURLs...)
}
