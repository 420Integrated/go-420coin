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

package vm

import (
	"fmt"
	"sort"

	"github.com/420integrated/go-420coin/params"
	"github.com/holiman/uint256"
)

var activators = map[int]func(*JumpTable){
	2929: enable2929,
	2200: enable2200,
	1884: enable1884,
	1344: enable1344,
	2315: enable2315,
}

// EnableEIP enables the given EIP on the config.
// This operation writes in-place, and callers need to ensure that the globally
// defined jump tables are not polluted.
func EnableEIP(eipNum int, jt *JumpTable) error {
	enablerFn, ok := activators[eipNum]
	if !ok {
		return fmt.Errorf("undefined eip %d", eipNum)
	}
	enablerFn(jt)
	return nil
}

func ValidEip(eipNum int) bool {
	_, ok := activators[eipNum]
	return ok
}
func ActivateableEips() []string {
	var nums []string
	for k := range activators {
		nums = append(nums, fmt.Sprintf("%d", k))
	}
	sort.Strings(nums)
	return nums
}

// enable1884 applies EIP-1884 to the given jump table:
// - Increase cost of BALANCE to 700
// - Increase cost of EXTCODEHASH to 700
// - Increase cost of SLOAD to 800
// - Define SELFBALANCE, with cost SmokeFastStep (5)
func enable1884(jt *JumpTable) {
	// Smoke cost changes
	jt[SLOAD].constantSmoke = params.SloadSmokeEIP1884
	jt[BALANCE].constantSmoke = params.BalanceSmokeEIP1884
	jt[EXTCODEHASH].constantSmoke = params.ExtcodeHashSmokeEIP1884

	// New opcode
	jt[SELFBALANCE] = &operation{
		execute:     opSelfBalance,
		constantSmoke: SmokeFastStep,
		minStack:    minStack(0, 1),
		maxStack:    maxStack(0, 1),
	}
}

func opSelfBalance(pc *uint64, interpreter *EVMInterpreter, callContext *callCtx) ([]byte, error) {
	balance, _ := uint256.FromBig(interpreter.evm.StateDB.GetBalance(callContext.contract.Address()))
	callContext.stack.push(balance)
	return nil, nil
}

// enable1344 applies EIP-1344 (ChainID Opcode)
// - Adds an opcode that returns the current chain’s EIP-155 unique identifier
func enable1344(jt *JumpTable) {
	// New opcode
	jt[CHAINID] = &operation{
		execute:     opChainID,
		constantSmoke: SmokeQuickStep,
		minStack:    minStack(0, 1),
		maxStack:    maxStack(0, 1),
	}
}

// opChainID implements CHAINID opcode
func opChainID(pc *uint64, interpreter *EVMInterpreter, callContext *callCtx) ([]byte, error) {
	chainId, _ := uint256.FromBig(interpreter.evm.chainConfig.ChainID)
	callContext.stack.push(chainId)
	return nil, nil
}

// enable2200 applies EIP-2200 (Rebalance net-metered SSTORE)
func enable2200(jt *JumpTable) {
	jt[SLOAD].constantSmoke = params.SloadSmokeEIP2200
	jt[SSTORE].dynamicSmoke = smokeSStoreEIP2200
}

// enable2315 applies EIP-2315 (Simple Subroutines)
// - Adds opcodes that jump to and return from subroutines
func enable2315(jt *JumpTable) {
	// New opcode
	jt[BEGINSUB] = &operation{
		execute:     opBeginSub,
		constantSmoke: SmokeQuickStep,
		minStack:    minStack(0, 0),
		maxStack:    maxStack(0, 0),
	}
	// New opcode
	jt[JUMPSUB] = &operation{
		execute:     opJumpSub,
		constantSmoke: SmokeSlowStep,
		minStack:    minStack(1, 0),
		maxStack:    maxStack(1, 0),
		jumps:       true,
	}
	// New opcode
	jt[RETURNSUB] = &operation{
		execute:     opReturnSub,
		constantSmoke: SmokeFastStep,
		minStack:    minStack(0, 0),
		maxStack:    maxStack(0, 0),
		jumps:       true,
	}
}

// enable2929 enables "EIP-2929: Smoke cost increases for state access opcodes"
// https://eips.ethereum.org/EIPS/eip-2929
func enable2929(jt *JumpTable) {
	jt[SSTORE].dynamicSmoke = smokeSStoreEIP2929

	jt[SLOAD].constantSmoke = 0
	jt[SLOAD].dynamicSmoke = smokeSLoadEIP2929

	jt[EXTCODECOPY].constantSmoke = WarmStorageReadCostEIP2929
	jt[EXTCODECOPY].dynamicSmoke = smokeExtCodeCopyEIP2929

	jt[EXTCODESIZE].constantSmoke = WarmStorageReadCostEIP2929
	jt[EXTCODESIZE].dynamicSmoke = smokeEip2929AccountCheck

	jt[EXTCODEHASH].constantSmoke = WarmStorageReadCostEIP2929
	jt[EXTCODEHASH].dynamicSmoke = smokeEip2929AccountCheck

	jt[BALANCE].constantSmoke = WarmStorageReadCostEIP2929
	jt[BALANCE].dynamicSmoke = smokeEip2929AccountCheck

	jt[CALL].constantSmoke = WarmStorageReadCostEIP2929
	jt[CALL].dynamicSmoke = smokeCallEIP2929

	jt[CALLCODE].constantSmoke = WarmStorageReadCostEIP2929
	jt[CALLCODE].dynamicSmoke = smokeCallCodeEIP2929

	jt[STATICCALL].constantSmoke = WarmStorageReadCostEIP2929
	jt[STATICCALL].dynamicSmoke = smokeStaticCallEIP2929

	jt[DELEGATECALL].constantSmoke = WarmStorageReadCostEIP2929
	jt[DELEGATECALL].dynamicSmoke = smokeDelegateCallEIP2929

	// This was previously part of the dynamic cost, but we're using it as a constantSmoke
	// factor here
	jt[SELFDESTRUCT].constantSmoke = params.SelfdestructSmokeEIP150
	jt[SELFDESTRUCT].dynamicSmoke = smokeSelfdestructEIP2929
}
