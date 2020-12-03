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

package vm

import (
	"errors"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/math"
	"github.com/420integrated/go-420coin/params"
)

// memorySmokeCost calculates the quadratic smoke for memory expansion. It does so
// only for the memory region that is expanded, not the total memory.
func memorySmokeCost(mem *Memory, newMemSize uint64) (uint64, error) {
	if newMemSize == 0 {
		return 0, nil
	}
	// The maximum that will fit in a uint64 is max_word_count - 1. Anything above
	// that will result in an overflow. Additionally, a newMemSize which results in
	// a newMemSizeWords larger than 0xFFFFFFFF will cause the square operation to
	// overflow. The constant 0x1FFFFFFFE0 is the highest number that can be used
	// without overflowing the smoke calculation.
	if newMemSize > 0x1FFFFFFFE0 {
		return 0, ErrSmokeUintOverflow
	}
	newMemSizeWords := toWordSize(newMemSize)
	newMemSize = newMemSizeWords * 32

	if newMemSize > uint64(mem.Len()) {
		square := newMemSizeWords * newMemSizeWords
		linCoef := newMemSizeWords * params.MemorySmoke
		quadCoef := square / params.QuadCoeffDiv
		newTotalFee := linCoef + quadCoef

		fee := newTotalFee - mem.lastSmokeCost
		mem.lastSmokeCost = newTotalFee

		return fee, nil
	}
	return 0, nil
}

// memoryCopierSmoke creates the smoke functions for the following opcodes, and takes
// the stack position of the operand which determines the size of the data to copy
// as argument:
// CALLDATACOPY (stack position 2)
// CODECOPY (stack position 2)
// EXTCODECOPY (stack poition 3)
// RETURNDATACOPY (stack position 2)
func memoryCopierSmoke(stackpos int) smokeFunc {
	return func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		// Smoke for expanding the memory
		smoke, err := memorySmokeCost(mem, memorySize)
		if err != nil {
			return 0, err
		}
		// And smoke for copying data, charged per word at param.CopySmoke
		words, overflow := stack.Back(stackpos).Uint64WithOverflow()
		if overflow {
			return 0, ErrSmokeUintOverflow
		}

		if words, overflow = math.SafeMul(toWordSize(words), params.CopySmoke); overflow {
			return 0, ErrSmokeUintOverflow
		}

		if smoke, overflow = math.SafeAdd(smoke, words); overflow {
			return 0, ErrSmokeUintOverflow
		}
		return smoke, nil
	}
}

var (
	smokeCallDataCopy   = memoryCopierSmoke(2)
	smokeCodeCopy       = memoryCopierSmoke(2)
	smokeExtCodeCopy    = memoryCopierSmoke(3)
	smokeReturnDataCopy = memoryCopierSmoke(2)
)

func smokeSStore(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		y, x    = stack.Back(1), stack.Back(0)
		current = evm.StateDB.GetState(contract.Address(), x.Bytes32())
	)
	// The legacy smoke metering only takes into consideration the current state
	// Legacy rules should be applied if we are in Petersburg (removal of EIP-1283)
	// OR Constantinople is not active
	if evm.chainRules.IsPetersburg || !evm.chainRules.IsConstantinople {
		// This checks for 3 scenario's and calculates smoke accordingly:
		//
		// 1. From a zero-value address to a non-zero value         (NEW VALUE)
		// 2. From a non-zero value address to a zero-value address (DELETE)
		// 3. From a non-zero to a non-zero                         (CHANGE)
		switch {
		case current == (common.Hash{}) && y.Sign() != 0: // 0 => non 0
			return params.SstoreSetSmoke, nil
		case current != (common.Hash{}) && y.Sign() == 0: // non 0 => 0
			evm.StateDB.AddRefund(params.SstoreRefundSmoke)
			return params.SstoreClearSmoke, nil
		default: // non 0 => non 0 (or 0 => 0)
			return params.SstoreResetSmoke, nil
		}
	}
	// The new smoke metering is based on net smoke costs (EIP-1283):
	//
	// 1. If current value equals new value (this is a no-op), 200 smoke is deducted.
	// 2. If current value does not equal new value
	//   2.1. If original value equals current value (this storage slot has not been changed by the current execution context)
	//     2.1.1. If original value is 0, 20000 smoke is deducted.
	// 	   2.1.2. Otherwise, 5000 smoke is deducted. If new value is 0, add 15000 smoke to refund counter.
	// 	2.2. If original value does not equal current value (this storage slot is dirty), 200 smoke is deducted. Apply both of the following clauses.
	// 	  2.2.1. If original value is not 0
	//       2.2.1.1. If current value is 0 (also means that new value is not 0), remove 15000 smoke from refund counter. We can prove that refund counter will never go below 0.
	//       2.2.1.2. If new value is 0 (also means that current value is not 0), add 15000 smoke to refund counter.
	// 	  2.2.2. If original value equals new value (this storage slot is reset)
	//       2.2.2.1. If original value is 0, add 19800 smoke to refund counter.
	// 	     2.2.2.2. Otherwise, add 4800 smoke to refund counter.
	value := common.Hash(y.Bytes32())
	if current == value { // noop (1)
		return params.NetSstoreNoopSmoke, nil
	}
	original := evm.StateDB.GetCommittedState(contract.Address(), x.Bytes32())
	if original == current {
		if original == (common.Hash{}) { // create slot (2.1.1)
			return params.NetSstoreInitSmoke, nil
		}
		if value == (common.Hash{}) { // delete slot (2.1.2b)
			evm.StateDB.AddRefund(params.NetSstoreClearRefund)
		}
		return params.NetSstoreCleanSmoke, nil // write existing slot (2.1.2)
	}
	if original != (common.Hash{}) {
		if current == (common.Hash{}) { // recreate slot (2.2.1.1)
			evm.StateDB.SubRefund(params.NetSstoreClearRefund)
		} else if value == (common.Hash{}) { // delete slot (2.2.1.2)
			evm.StateDB.AddRefund(params.NetSstoreClearRefund)
		}
	}
	if original == value {
		if original == (common.Hash{}) { // reset to original inexistent slot (2.2.2.1)
			evm.StateDB.AddRefund(params.NetSstoreResetClearRefund)
		} else { // reset to original existing slot (2.2.2.2)
			evm.StateDB.AddRefund(params.NetSstoreResetRefund)
		}
	}
	return params.NetSstoreDirtySmoke, nil
}

// 0. If *smokeleft* is less than or equal to 2300, fail the current call.
// 1. If current value equals new value (this is a no-op), SLOAD_SMOKE smoke is deducted.
// 2. If current value does not equal new value:
//   2.1. If original value equals current value (this storage slot has not been changed by the current execution context):
//     2.1.1. If original value is 0, SSTORE_SET_SMOKE (20K) smoke is deducted.
//     2.1.2. Otherwise, SSTORE_RESET_SMOKE SMOKE is deducted. If new value is 0, add SSTORE_CLEARS_SCHEDULE to refund counter.
//   2.2. If original value does not equal current value (this storage slot is dirty), SLOAD_SMOKE SMOKE is deducted. Apply both of the following clauses:
//     2.2.1. If original value is not 0:
//       2.2.1.1. If current value is 0 (also means that new value is not 0), subtract SSTORE_CLEARS_SCHEDULE smoke from refund counter.
//       2.2.1.2. If new value is 0 (also means that current value is not 0), add SSTORE_CLEARS_SCHEDULE smoke to refund counter.
//     2.2.2. If original value equals new value (this storage slot is reset):
//       2.2.2.1. If original value is 0, add SSTORE_SET_SMOKE - SLOAD_SMOKE to refund counter.
//       2.2.2.2. Otherwise, add SSTORE_RESET_SMOKE - SLOAD_SMOKE smoke to refund counter.
func smokeSStoreEIP2200(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	// If we fail the minimum smoke availability invariant, fail (0)
	if contract.Smoke <= params.SstoreSentrySmokeEIP2200 {
		return 0, errors.New("not enough smoke for reentrancy sentry")
	}
	// Smoke sentry honoured, do the actual smoke calculation based on the stored value
	var (
		y, x    = stack.Back(1), stack.Back(0)
		current = evm.StateDB.GetState(contract.Address(), x.Bytes32())
	)
	value := common.Hash(y.Bytes32())

	if current == value { // noop (1)
		return params.SloadSmokeEIP2200, nil
	}
	original := evm.StateDB.GetCommittedState(contract.Address(), x.Bytes32())
	if original == current {
		if original == (common.Hash{}) { // create slot (2.1.1)
			return params.SstoreSetSmokeEIP2200, nil
		}
		if value == (common.Hash{}) { // delete slot (2.1.2b)
			evm.StateDB.AddRefund(params.SstoreClearsScheduleRefundEIP2200)
		}
		return params.SstoreResetSmokeEIP2200, nil // write existing slot (2.1.2)
	}
	if original != (common.Hash{}) {
		if current == (common.Hash{}) { // recreate slot (2.2.1.1)
			evm.StateDB.SubRefund(params.SstoreClearsScheduleRefundEIP2200)
		} else if value == (common.Hash{}) { // delete slot (2.2.1.2)
			evm.StateDB.AddRefund(params.SstoreClearsScheduleRefundEIP2200)
		}
	}
	if original == value {
		if original == (common.Hash{}) { // reset to original inexistent slot (2.2.2.1)
			evm.StateDB.AddRefund(params.SstoreSetSmokeEIP2200 - params.SloadSmokeEIP2200)
		} else { // reset to original existing slot (2.2.2.2)
			evm.StateDB.AddRefund(params.SstoreResetSmokeEIP2200 - params.SloadSmokeEIP2200)
		}
	}
	return params.SloadSmokeEIP2200, nil // dirty update (2.2)
}

func makeSmokeLog(n uint64) smokeFunc {
	return func(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
		requestedSize, overflow := stack.Back(1).Uint64WithOverflow()
		if overflow {
			return 0, ErrSmokeUintOverflow
		}

		smoke, err := memorySmokeCost(mem, memorySize)
		if err != nil {
			return 0, err
		}

		if smoke, overflow = math.SafeAdd(smoke, params.LogSmoke); overflow {
			return 0, ErrSmokeUintOverflow
		}
		if smoke, overflow = math.SafeAdd(smoke, n*params.LogTopicSmoke); overflow {
			return 0, ErrSmokeUintOverflow
		}

		var memorySizeSmoke uint64
		if memorySizeSmoke, overflow = math.SafeMul(requestedSize, params.LogDataSmoke); overflow {
			return 0, ErrSmokeUintOverflow
		}
		if smoke, overflow = math.SafeAdd(smoke, memorySizeSmoke); overflow {
			return 0, ErrSmokeUintOverflow
		}
		return smoke, nil
	}
}

func smokeSha3(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	smoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	wordSmoke, overflow := stack.Back(1).Uint64WithOverflow()
	if overflow {
		return 0, ErrSmokeUintOverflow
	}
	if wordSmoke, overflow = math.SafeMul(toWordSize(wordSmoke), params.Sha3WordSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	if smoke, overflow = math.SafeAdd(smoke, wordSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

// pureMemorySmokecost is used by several operations, which aside from their
// static cost have a dynamic cost which is solely based on the memory
// expansion
func pureMemorySmokecost(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	return memorySmokeCost(mem, memorySize)
}

var (
	smokeReturn  = pureMemorySmokecost
	smokeRevert  = pureMemorySmokecost
	smokeMLoad   = pureMemorySmokecost
	smokeMStore8 = pureMemorySmokecost
	smokeMStore  = pureMemorySmokecost
	smokeCreate  = pureMemorySmokecost
)

func smokeCreate2(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	smoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	wordSmoke, overflow := stack.Back(2).Uint64WithOverflow()
	if overflow {
		return 0, ErrSmokeUintOverflow
	}
	if wordSmoke, overflow = math.SafeMul(toWordSize(wordSmoke), params.Sha3WordSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	if smoke, overflow = math.SafeAdd(smoke, wordSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeExpFrontier(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	expByteLen := uint64((stack.data[stack.len()-2].BitLen() + 7) / 8)

	var (
		smoke      = expByteLen * params.ExpByteFrontier // no overflow check required. Max is 256 * ExpByte smoke
		overflow bool
	)
	if smoke, overflow = math.SafeAdd(smoke, params.ExpSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeExpEIP158(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	expByteLen := uint64((stack.data[stack.len()-2].BitLen() + 7) / 8)

	var (
		smoke      = expByteLen * params.ExpByteEIP158 // no overflow check required. Max is 256 * ExpByte smoke
		overflow bool
	)
	if smoke, overflow = math.SafeAdd(smoke, params.ExpSmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeCall(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var (
		smoke            uint64
		transfersValue = !stack.Back(2).IsZero()
		address        = common.Address(stack.Back(1).Bytes20())
	)
	if evm.chainRules.IsEIP158 {
		if transfersValue && evm.StateDB.Empty(address) {
			smoke += params.CallNewAccountSmoke
		}
	} else if !evm.StateDB.Exist(address) {
		smoke += params.CallNewAccountSmoke
	}
	if transfersValue {
		smoke += params.CallValueTransferSmoke
	}
	memorySmoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var overflow bool
	if smoke, overflow = math.SafeAdd(smoke, memorySmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}

	evm.callSmokeTemp, err = callSmoke(evm.chainRules.IsEIP150, contract.Smoke, smoke, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if smoke, overflow = math.SafeAdd(smoke, evm.callSmokeTemp); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeCallCode(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	memorySmoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	var (
		smoke      uint64
		overflow bool
	)
	if stack.Back(2).Sign() != 0 {
		smoke += params.CallValueTransferSmoke
	}
	if smoke, overflow = math.SafeAdd(smoke, memorySmoke); overflow {
		return 0, ErrSmokeUintOverflow
	}
	evm.callSmokeTemp, err = callSmoke(evm.chainRules.IsEIP150, contract.Smoke, smoke, stack.Back(0))
	if err != nil {
		return 0, err
	}
	if smoke, overflow = math.SafeAdd(smoke, evm.callSmokeTemp); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeDelegateCall(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	smoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	evm.callSmokeTemp, err = callSmoke(evm.chainRules.IsEIP150, contract.Smoke, smoke, stack.Back(0))
	if err != nil {
		return 0, err
	}
	var overflow bool
	if smoke, overflow = math.SafeAdd(smoke, evm.callSmokeTemp); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeStaticCall(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	smoke, err := memorySmokeCost(mem, memorySize)
	if err != nil {
		return 0, err
	}
	evm.callSmokeTemp, err = callSmoke(evm.chainRules.IsEIP150, contract.Smoke, smoke, stack.Back(0))
	if err != nil {
		return 0, err
	}
	var overflow bool
	if smoke, overflow = math.SafeAdd(smoke, evm.callSmokeTemp); overflow {
		return 0, ErrSmokeUintOverflow
	}
	return smoke, nil
}

func smokeSelfdestruct(evm *EVM, contract *Contract, stack *Stack, mem *Memory, memorySize uint64) (uint64, error) {
	var smoke uint64
	// EIP150 homestead smoke reprice fork:
	if evm.chainRules.IsEIP150 {
		smoke = params.SelfdestructSmokeEIP150
		var address = common.Address(stack.Back(0).Bytes20())

		if evm.chainRules.IsEIP158 {
			// if empty and transfers value
			if evm.StateDB.Empty(address) && evm.StateDB.GetBalance(contract.Address()).Sign() != 0 {
				smoke += params.CreateBySelfdestructSmoke
			}
		} else if !evm.StateDB.Exist(address) {
			smoke += params.CreateBySelfdestructSmoke
		}
	}

	if !evm.StateDB.HasSuicided(contract.Address()) {
		evm.StateDB.AddRefund(params.SelfdestructRefundSmoke)
	}
	return smoke, nil
}
