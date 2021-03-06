// Copyright 2017 The The 420Integrated Development Group
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

package params

// These are the multipliers for 420coin denominations.
// Example: To get the marley value of an amount in 'mahers', use
//
//    new(big.Int).Mul(value, big.NewInt(params.GMarley))
//
const (
	Marley   = 1
	Maher  = 1e9
	Fourtwentycoin = 1e18
)
