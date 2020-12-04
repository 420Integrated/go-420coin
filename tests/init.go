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

package tests

import (
	"fmt"
	"math/big"
	"sort"

	"github.com/420integrated/go-420coin/params"
)

// Forks table defines supported forks and their chain config.
var Forks = map[string]*params.ChainConfig{
	"Frontier": {
		ChainID: big.NewInt(2020),
	},
	"Homestead": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(1),
	},
	"EIP150": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(1),
		EIP150Block:    big.NewInt(2),
	},
	"EIP158": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(1),
		EIP150Block:    big.NewInt(2),
		EIP155Block:    big.NewInt(3),
		EIP158Block:    big.NewInt(3),
	},
	"Byzantium": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(1),
		EIP150Block:    big.NewInt(2),
		EIP155Block:    big.NewInt(3),
		EIP158Block:    big.NewInt(3),
		DAOForkBlock:   big.NewInt(1),
		ByzantiumBlock: big.NewInt(4),
	},
	"Constantinople": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(1),
		EIP150Block:         big.NewInt(2),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		DAOForkBlock:        big.NewInt(2),
		ByzantiumBlock:      big.NewInt(4),
		ConstantinopleBlock: big.NewInt(5),
		PetersburgBlock:     big.NewInt(6),
	},
	"ConstantinopleFix": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(1),
		EIP150Block:         big.NewInt(2),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		DAOForkBlock:        big.NewInt(2),
		ByzantiumBlock:      big.NewInt(4),
		ConstantinopleBlock: big.NewInt(5),
		PetersburgBlock:     big.NewInt(6),
	},
	"Istanbul": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(1),
		EIP150Block:         big.NewInt(2),
		EIP155Block:         big.NewInt(3),
		EIP158Block:         big.NewInt(3),
		DAOForkBlock:        big.NewInt(2),
		ByzantiumBlock:      big.NewInt(4),
		ConstantinopleBlock: big.NewInt(5),
		PetersburgBlock:     big.NewInt(6),
		IstanbulBlock:       big.NewInt(7),
	},
	"FrontierToHomesteadAt5": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(5),
	},
	"HomesteadToEIP150At5": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(0),
		EIP150Block:    big.NewInt(5),
	},
	"HomesteadToDaoAt5": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(0),
		DAOForkBlock:   big.NewInt(5),
		DAOForkSupport: true,
	},
	"EIP158ToByzantiumAt5": {
		ChainID:        big.NewInt(2020),
		HomesteadBlock: big.NewInt(0),
		EIP150Block:    big.NewInt(0),
		EIP155Block:    big.NewInt(0),
		EIP158Block:    big.NewInt(0),
		ByzantiumBlock: big.NewInt(5),
	},
	"ByzantiumToConstantinopleAt5": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(5),
	},
	"ByzantiumToConstantinopleFixAt5": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(5),
		PetersburgBlock:     big.NewInt(5),
	},
	"ConstantinopleFixToIstanbulAt5": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(5),
	},
	"YOLOv2": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		YoloV2Block:         big.NewInt(0),
	},
	// This specification is subject to change, but is for now identical to YOLOv2
	// for cross-client testing purposes
	"Berlin": {
		ChainID:             big.NewInt(2020),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		YoloV2Block:         big.NewInt(0),
	},
}

// Returns the set of defined fork names
func AvailableForks() []string {
	var availableForks []string
	for k := range Forks {
		availableForks = append(availableForks, k)
	}
	sort.Strings(availableForks)
	return availableForks
}

// UnsupportedForkError is returned when a test requests a fork that isn't implemented.
type UnsupportedForkError struct {
	Name string
}

func (e UnsupportedForkError) Error() string {
	return fmt.Sprintf("unsupported fork %q", e.Name)
}
