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

package vm

import (
	"github.com/holiman/uint256"
)

// Smoke costs
const (
	SmokeQuickStep   uint64 = 2
	SmokeFastestStep uint64 = 3
	SmokeFastStep    uint64 = 5
	SmokeMidStep     uint64 = 8
	SmokeSlowStep    uint64 = 10
	SmokeExtStep     uint64 = 20
)

// callSmoke returns the actual smoke cost of the call.
//
// The cost of smoke was changed during the homestead price change HF.
// As part of EIP 150 (TangerineWhistle), the returned smoke is smoke - base * 63 / 64.
func callSmoke(isEip150 bool, availableSmoke, base uint64, callCost *uint256.Int) (uint64, error) {
	if isEip150 {
		availableSmoke = availableSmoke - base
		smoke := availableSmoke - availableSmoke/64
		// If the bit length exceeds 64 bit we know that the newly calculated "smoke" for EIP150
		// is smaller than the requested amount. Therefore we return the new smoke instead
		// of returning an error.
		if !callCost.IsUint64() || smoke < callCost.Uint64() {
			return smoke, nil
		}
	}
	if !callCost.IsUint64() {
		return 0, ErrSmokeUintOverflow
	}

	return callCost.Uint64(), nil
}
