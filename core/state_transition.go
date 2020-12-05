// Copyright 2014 The The 420Integrated Development Group
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
	"math/big"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/core/vm"
	"github.com/420integrated/go-420coin/params"
)

/*
The State Transitioning Model

A state transition is a change made when a transaction is applied to the current world state
The state transitioning model does all the necessary work to work out a valid new state root.

1) Nonce handling
2) Pre pay smoke
3) Create a new state object if the recipient is \0*32
4) Value transfer
== If contract creation ==
  4a) Attempt to run transaction data
  4b) If valid, use result as code for the new state object
== end ==
5) Run Script section
6) Derive new state root
*/
type StateTransition struct {
	gp         *SmokePool
	msg        Message
	smoke        uint64
	smokePrice   *big.Int
	initialSmoke uint64
	value      *big.Int
	data       []byte
	state      vm.StateDB
	evm        *vm.EVM
}

// Message represents a message sent to a contract.
type Message interface {
	From() common.Address
	To() *common.Address

	SmokePrice() *big.Int
	Smoke() uint64
	Value() *big.Int

	Nonce() uint64
	CheckNonce() bool
	Data() []byte
}

// ExecutionResult includes all output after executing given evm
// message no matter the execution itself is successful or not.
type ExecutionResult struct {
	UsedSmoke    uint64 // Total used smoke but include the refunded smoke
	Err        error  // Any error encountered during the execution(listed in core/vm/errors.go)
	ReturnData []byte // Returned data from evm(function result or data supplied with revert opcode)
}

// Unwrap returns the internal evm error which allows us for further
// analysis outside.
func (result *ExecutionResult) Unwrap() error {
	return result.Err
}

// Failed returns the indicator if the execution is successful or not
func (result *ExecutionResult) Failed() bool { return result.Err != nil }

// Return is a helper function to help caller distinguish between revert reason
// and function return. Return returns the data after execution if no error occurs.
func (result *ExecutionResult) Return() []byte {
	if result.Err != nil {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// Revert returns the concrete revert reason if the execution is aborted by `REVERT`
// opcode. Note the reason can be nil if no data supplied with revert opcode.
func (result *ExecutionResult) Revert() []byte {
	if result.Err != vm.ErrExecutionReverted {
		return nil
	}
	return common.CopyBytes(result.ReturnData)
}

// IntrinsicSmoke computes the 'intrinsic smoke' for a message with the given data.
func IntrinsicSmoke(data []byte, contractCreation, isHomestead bool, isEIP2028 bool) (uint64, error) {
	// Set the starting smoke for the raw transaction
	var smoke uint64
	if contractCreation && isHomestead {
		smoke = params.TxSmokeContractCreation
	} else {
		smoke = params.TxSmoke
	}
	// Bump the required smoke by the amount of transactional data
	if len(data) > 0 {
		// Zero and non-zero bytes are priced differently
		var nz uint64
		for _, byt := range data {
			if byt != 0 {
				nz++
			}
		}
		// Make sure we don't exceed uint64 for all data combinations
		nonZeroSmoke := params.TxDataNonZeroSmokeFrontier
		if isEIP2028 {
			nonZeroSmoke = params.TxDataNonZeroSmokeEIP2028
		}
		if (math.MaxUint64-smoke)/nonZeroSmoke < nz {
			return 0, ErrSmokeUintOverflow
		}
		smoke += nz * nonZeroSmoke

		z := uint64(len(data)) - nz
		if (math.MaxUint64-smoke)/params.TxDataZeroSmoke < z {
			return 0, ErrSmokeUintOverflow
		}
		smoke += z * params.TxDataZeroSmoke
	}
	return smoke, nil
}

// NewStateTransition initialises and returns a new state transition object.
func NewStateTransition(evm *vm.EVM, msg Message, gp *SmokePool) *StateTransition {
	return &StateTransition{
		gp:       gp,
		evm:      evm,
		msg:      msg,
		smokePrice: msg.SmokePrice(),
		value:    msg.Value(),
		data:     msg.Data(),
		state:    evm.StateDB,
	}
}

// ApplyMessage computes the new state by applying the given message
// against the old state within the environment.
//
// ApplyMessage returns the bytes returned by any EVM execution (if it took place),
// the smoke used (which includes smoke refunds) and an error if it failed. An error always
// indicates a core error meaning that the message would always fail for that particular
// state and would never be accepted within a block.
func ApplyMessage(evm *vm.EVM, msg Message, gp *SmokePool) (*ExecutionResult, error) {
	return NewStateTransition(evm, msg, gp).TransitionDb()
}

// to returns the recipient of the message.
func (st *StateTransition) to() common.Address {
	if st.msg == nil || st.msg.To() == nil /* contract creation */ {
		return common.Address{}
	}
	return *st.msg.To()
}

func (st *StateTransition) buySmoke() error {
	mgval := new(big.Int).Mul(new(big.Int).SetUint64(st.msg.Smoke()), st.smokePrice)
	if have, want := st.state.GetBalance(st.msg.From()), mgval; have.Cmp(want) < 0 {
		return fmt.Errorf("%w: address %v have %v want %v", ErrInsufficientFunds, st.msg.From().Hex(), have, want)
	}
	if err := st.gp.SubSmoke(st.msg.Smoke()); err != nil {
		return err
	}
	st.smoke += st.msg.Smoke()

	st.initialSmoke = st.msg.Smoke()
	st.state.SubBalance(st.msg.From(), mgval)
	return nil
}

func (st *StateTransition) preCheck() error {
	// Make sure this transaction's nonce is correct.
	if st.msg.CheckNonce() {
		stNonce := st.state.GetNonce(st.msg.From())
		if msgNonce := st.msg.Nonce(); stNonce < msgNonce {
			return fmt.Errorf("%w: address %v, tx: %d state: %d", ErrNonceTooHigh,
				st.msg.From().Hex(), msgNonce, stNonce)
		} else if stNonce > msgNonce {
			return fmt.Errorf("%w: address %v, tx: %d state: %d", ErrNonceTooLow,
				st.msg.From().Hex(), msgNonce, stNonce)
		}
	}
	return st.buySmoke()
}

// TransitionDb will transition the state by applying the current message and
// returning the evm execution result with following fields.
//
// - used smoke:
//      total smoke used (including smoke being refunded)
// - returndata:
//      the returned data from evm
// - concrete execution error:
//      various **EVM** error which aborts the execution,
//      e.g. ErrOutOfSmoke, ErrExecutionReverted
//
// However if any consensus issue encountered, return the error directly with
// nil evm execution result.
func (st *StateTransition) TransitionDb() (*ExecutionResult, error) {
	// First check this message satisfies all consensus rules before
	// applying the message. The rules include these clauses
	//
	// 1. the nonce of the message caller is correct
	// 2. caller has enough balance to cover transaction fee(smokelimit * smokeprice)
	// 3. the amount of smoke required is available in the block
	// 4. the purchased smoke is enough to cover intrinsic usage
	// 5. there is no overflow when calculating intrinsic smoke
	// 6. caller has enough balance to cover asset transfer for **topmost** call

	// Check clauses 1-3, buy smoke if everything is correct
	if err := st.preCheck(); err != nil {
		return nil, err
	}
	msg := st.msg
	sender := vm.AccountRef(msg.From())
	homestead := st.evm.ChainConfig().IsHomestead(st.evm.Context.BlockNumber)
	istanbul := st.evm.ChainConfig().IsIstanbul(st.evm.Context.BlockNumber)
	contractCreation := msg.To() == nil

	// Check clauses 4-5, subtract intrinsic smoke if everything is correct
	smoke, err := IntrinsicSmoke(st.data, contractCreation, homestead, istanbul)
	if err != nil {
		return nil, err
	}
	if st.smoke < smoke {
		return nil, fmt.Errorf("%w: have %d, want %d", ErrIntrinsicSmoke, st.smoke, smoke)
	}
	st.smoke -= smoke

	// Check clause 6
	if msg.Value().Sign() > 0 && !st.evm.Context.CanTransfer(st.state, msg.From(), msg.Value()) {
		return nil, fmt.Errorf("%w: address %v", ErrInsufficientFundsForTransfer, msg.From().Hex())
	}
	var (
		ret   []byte
		vmerr error // vm errors do not effect consensus and are therefore not assigned to err
	)
	if contractCreation {
		ret, _, st.smoke, vmerr = st.evm.Create(sender, st.data, st.smoke, st.value)
	} else {
		// Increment the nonce for the next transaction
		st.state.SetNonce(msg.From(), st.state.GetNonce(sender.Address())+1)
		ret, st.smoke, vmerr = st.evm.Call(sender, st.to(), st.data, st.smoke, st.value)
	}
	st.refundSmoke()
	st.state.AddBalance(st.evm.Context.Coinbase, new(big.Int).Mul(new(big.Int).SetUint64(st.smokeUsed()), st.smokePrice))

	return &ExecutionResult{
		UsedSmoke:    st.smokeUsed(),
		Err:        vmerr,
		ReturnData: ret,
	}, nil
}

func (st *StateTransition) refundSmoke() {
	// Apply refund counter, capped to half of the used smoke.
	refund := st.smokeUsed() / 2
	if refund > st.state.GetRefund() {
		refund = st.state.GetRefund()
	}
	st.smoke += refund

	// Return 420 for remaining smoke, exchanged at the original rate.
	remaining := new(big.Int).Mul(new(big.Int).SetUint64(st.smoke), st.smokePrice)
	st.state.AddBalance(st.msg.From(), remaining)

	// Also return remaining smoke to the block smoke counter so it is
	// available for the next transaction.
	st.gp.AddSmoke(st.smoke)
}

// smokeUsed returns the amount of smoke used up by the state transition.
func (st *StateTransition) smokeUsed() uint64 {
	return st.initialSmoke - st.smoke
}
