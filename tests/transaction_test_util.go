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

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/hexutil"
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/params"
	"github.com/420integrated/go-420coin/rlp"
)

// TransactionTest checks RLP decoding and sender derivation of transactions.
type TransactionTest struct {
	RLP            hexutil.Bytes `json:"rlp"`
	Byzantium      ttFork
	Constantinople ttFork
	Istanbul       ttFork
	EIP150         ttFork
	EIP158         ttFork
	Frontier       ttFork
	Homestead      ttFork
}

type ttFork struct {
	Sender common.UnprefixedAddress `json:"sender"`
	Hash   common.UnprefixedHash    `json:"hash"`
}

func (tt *TransactionTest) Run(config *params.ChainConfig) error {
	validateTx := func(rlpData hexutil.Bytes, signer types.Signer, isHomestead bool, isIstanbul bool) (*common.Address, *common.Hash, error) {
		tx := new(types.Transaction)
		if err := rlp.DecodeBytes(rlpData, tx); err != nil {
			return nil, nil, err
		}
		sender, err := types.Sender(signer, tx)
		if err != nil {
			return nil, nil, err
		}
		// Intrinsic smoke
		requiredSmoke, err := core.IntrinsicSmoke(tx.Data(), tx.To() == nil, isHomestead, isIstanbul)
		if err != nil {
			return nil, nil, err
		}
		if requiredSmoke > tx.Smoke() {
			return nil, nil, fmt.Errorf("insufficient smoke ( %d < %d )", tx.Smoke(), requiredSmoke)
		}
		h := tx.Hash()
		return &sender, &h, nil
	}

	for _, testcase := range []struct {
		name        string
		signer      types.Signer
		fork        ttFork
		isHomestead bool
		isIstanbul  bool
	}{
		{"Frontier", types.FrontierSigner{}, tt.Frontier, false, false},
		{"Homestead", types.HomesteadSigner{}, tt.Homestead, true, false},
		{"EIP150", types.HomesteadSigner{}, tt.EIP150, true, false},
		{"EIP158", types.NewEIP155Signer(config.ChainID), tt.EIP158, true, false},
		{"Byzantium", types.NewEIP155Signer(config.ChainID), tt.Byzantium, true, false},
		{"Constantinople", types.NewEIP155Signer(config.ChainID), tt.Constantinople, true, false},
		{"Istanbul", types.NewEIP155Signer(config.ChainID), tt.Istanbul, true, true},
	} {
		sender, txhash, err := validateTx(tt.RLP, testcase.signer, testcase.isHomestead, testcase.isIstanbul)

		if testcase.fork.Sender == (common.UnprefixedAddress{}) {
			if err == nil {
				return fmt.Errorf("expected error, got none (address %v)[%v]", sender.String(), testcase.name)
			}
			continue
		}
		// Should resolve the right address
		if err != nil {
			return fmt.Errorf("got error, expected none: %v", err)
		}
		if sender == nil {
			return fmt.Errorf("sender was nil, should be %x", common.Address(testcase.fork.Sender))
		}
		if *sender != common.Address(testcase.fork.Sender) {
			return fmt.Errorf("sender mismatch: got %x, want %x", sender, testcase.fork.Sender)
		}
		if txhash == nil {
			return fmt.Errorf("txhash was nil, should be %x", common.Hash(testcase.fork.Hash))
		}
		if *txhash != common.Hash(testcase.fork.Hash) {
			return fmt.Errorf("hash mismatch: got %x, want %x", *txhash, testcase.fork.Hash)
		}
	}
	return nil
}
