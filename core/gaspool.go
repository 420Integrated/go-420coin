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

package core

import (
	"fmt"
	"math"
)

// SmokePool tracks the amount of smoke available during execution of the transactions
// in a block. The zero value is a pool with zero smoke available.
type SmokePool uint64

// AddSmoke makes smoke available for execution.
func (gp *SmokePool) AddSmoke(amount uint64) *SmokePool {
	if uint64(*gp) > math.MaxUint64-amount {
		panic("smoke pool pushed above uint64")
	}
	*(*uint64)(gp) += amount
	return gp
}

// SubSmoke deducts the given amount from the pool if enough smoke is
// available and returns an error otherwise.
func (gp *SmokePool) SubSmoke(amount uint64) error {
	if uint64(*gp) < amount {
		return ErrSmokeLimitReached
	}
	*(*uint64)(gp) -= amount
	return nil
}

// Smoke returns the amount of smoke remaining in the pool.
func (gp *SmokePool) Smoke() uint64 {
	return uint64(*gp)
}

func (gp *SmokePool) String() string {
	return fmt.Sprintf("%d", *gp)
}
