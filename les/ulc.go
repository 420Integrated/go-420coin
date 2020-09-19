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

package les

import (
	"errors"

	"github.com/420integrated/go-420coin/log"
	"github.com/420integrated/go-420coin/p2p/enode"
)

type ulc struct {
	keys     map[string]bool
	fraction int
}

// newULC creates and returns an ultra light client instance.
func newULC(servers []string, fraction int) (*ulc, error) {
	keys := make(map[string]bool)
	for _, id := range servers {
		node, err := enode.Parse(enode.ValidSchemes, id)
		if err != nil {
			log.Warn("Failed to parse trusted server", "id", id, "err", err)
			continue
		}
		keys[node.ID().String()] = true
	}
	if len(keys) == 0 {
		return nil, errors.New("no trusted servers")
	}
	return &ulc{
		keys:     keys,
		fraction: fraction,
	}, nil
}

// trusted return an indicator that if the specified peer is trusted.
func (u *ulc) trusted(p enode.ID) bool {
	return u.keys[p.String()]
}
