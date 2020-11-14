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
	"github.com/420integrated/go-420coin/params"
)

type (
	executionFunc func(pc *uint64, interpreter *EVMInterpreter, callContext *callCtx) ([]byte, error)
	smokeFunc       func(*EVM, *Contract, *Stack, *Memory, uint64) (uint64, error) // last parameter is the requested memory size as a uint64
	// memorySizeFunc returns the required size, and if the operation overflowed a uint64
	memorySizeFunc func(*Stack) (size uint64, overflow bool)
)

type operation struct {
	// execute is the operation function
	execute     executionFunc
	constantSmoke uint64
	dynamicSmoke  smokeFunc
	// minStack tells how many stack items are required
	minStack int
	// maxStack specifies the max length the stack can have for this operation
	// to not overflow the stack.
	maxStack int

	// memorySize returns the memory size required for the operation
	memorySize memorySizeFunc

	halts   bool // indicates if the operation should halt further execution
	jumps   bool // indicates if the program counter should not increment
	writes  bool // determines if this a state modifying operation
	reverts bool // determines if the operation reverts state (implicitly halts)
	returns bool // determines if the operations sets the return data content
}

var (
	frontierInstructionSet         = newFrontierInstructionSet()
	homesteadInstructionSet        = newHomesteadInstructionSet()
	tangerineWhistleInstructionSet = newTangerineWhistleInstructionSet()
	spuriousDragonInstructionSet   = newSpuriousDragonInstructionSet()
	byzantiumInstructionSet        = newByzantiumInstructionSet()
	constantinopleInstructionSet   = newConstantinopleInstructionSet()
	istanbulInstructionSet         = newIstanbulInstructionSet()
	yoloV2InstructionSet           = newYoloV2InstructionSet()
)

// JumpTable contains the EVM opcodes supported at a given fork.
type JumpTable [256]*operation

// newYoloV2InstructionSet creates an instructionset containing
// - "EIP-2315: Simple Subroutines"
// - "EIP-2929: Smoke cost increases for state access opcodes"
func newYoloV2InstructionSet() JumpTable {
	instructionSet := newIstanbulInstructionSet()
	enable2315(&instructionSet) // Subroutines - https://eips.ethereum.org/EIPS/eip-2315
	enable2929(&instructionSet) // Access lists for trie accesses https://eips.ethereum.org/EIPS/eip-2929
	return instructionSet
}

// newIstanbulInstructionSet returns the frontier, homestead
// byzantium, contantinople and petersburg instructions.
func newIstanbulInstructionSet() JumpTable {
	instructionSet := newConstantinopleInstructionSet()

	enable1344(&instructionSet) // ChainID opcode - https://eips.ethereum.org/EIPS/eip-1344
	enable1884(&instructionSet) // Reprice reader opcodes - https://eips.ethereum.org/EIPS/eip-1884
	enable2200(&instructionSet) // Net metered SSTORE - https://eips.ethereum.org/EIPS/eip-2200

	return instructionSet
}

// newConstantinopleInstructionSet returns the frontier, homestead
// byzantium and contantinople instructions.
func newConstantinopleInstructionSet() JumpTable {
	instructionSet := newByzantiumInstructionSet()
	instructionSet[SHL] = &operation{
		execute:     opSHL,
		constantSmoke: SmokeFastestStep,
		minStack:    minStack(2, 1),
		maxStack:    maxStack(2, 1),
	}
	instructionSet[SHR] = &operation{
		execute:     opSHR,
		constantSmoke: SmokeFastestStep,
		minStack:    minStack(2, 1),
		maxStack:    maxStack(2, 1),
	}
	instructionSet[SAR] = &operation{
		execute:     opSAR,
		constantSmoke: SmokeFastestStep,
		minStack:    minStack(2, 1),
		maxStack:    maxStack(2, 1),
	}
	instructionSet[EXTCODEHASH] = &operation{
		execute:     opExtCodeHash,
		constantSmoke: params.ExtcodeHashSmokeConstantinople,
		minStack:    minStack(1, 1),
		maxStack:    maxStack(1, 1),
	}
	instructionSet[CREATE2] = &operation{
		execute:     opCreate2,
		constantSmoke: params.Create2Smoke,
		dynamicSmoke:  smokeCreate2,
		minStack:    minStack(4, 1),
		maxStack:    maxStack(4, 1),
		memorySize:  memoryCreate2,
		writes:      true,
		returns:     true,
	}
	return instructionSet
}

// newByzantiumInstructionSet returns the frontier, homestead and
// byzantium instructions.
func newByzantiumInstructionSet() JumpTable {
	instructionSet := newSpuriousDragonInstructionSet()
	instructionSet[STATICCALL] = &operation{
		execute:     opStaticCall,
		constantSmoke: params.CallSmokeEIP150,
		dynamicSmoke:  smokeStaticCall,
		minStack:    minStack(6, 1),
		maxStack:    maxStack(6, 1),
		memorySize:  memoryStaticCall,
		returns:     true,
	}
	instructionSet[RETURNDATASIZE] = &operation{
		execute:     opReturnDataSize,
		constantSmoke: SmokeQuickStep,
		minStack:    minStack(0, 1),
		maxStack:    maxStack(0, 1),
	}
	instructionSet[RETURNDATACOPY] = &operation{
		execute:     opReturnDataCopy,
		constantSmoke: SmokeFastestStep,
		dynamicSmoke:  smokeReturnDataCopy,
		minStack:    minStack(3, 0),
		maxStack:    maxStack(3, 0),
		memorySize:  memoryReturnDataCopy,
	}
	instructionSet[REVERT] = &operation{
		execute:    opRevert,
		dynamicSmoke: smokeRevert,
		minStack:   minStack(2, 0),
		maxStack:   maxStack(2, 0),
		memorySize: memoryRevert,
		reverts:    true,
		returns:    true,
	}
	return instructionSet
}

// EIP 158 a.k.a Spurious Dragon
func newSpuriousDragonInstructionSet() JumpTable {
	instructionSet := newTangerineWhistleInstructionSet()
	instructionSet[EXP].dynamicSmoke = smokeExpEIP158
	return instructionSet

}

// EIP 150 a.k.a Tangerine Whistle
func newTangerineWhistleInstructionSet() JumpTable {
	instructionSet := newHomesteadInstructionSet()
	instructionSet[BALANCE].constantSmoke = params.BalanceSmokeEIP150
	instructionSet[EXTCODESIZE].constantSmoke = params.ExtcodeSizeSmokeEIP150
	instructionSet[SLOAD].constantSmoke = params.SloadSmokeEIP150
	instructionSet[EXTCODECOPY].constantSmoke = params.ExtcodeCopyBaseEIP150
	instructionSet[CALL].constantSmoke = params.CallSmokeEIP150
	instructionSet[CALLCODE].constantSmoke = params.CallSmokeEIP150
	instructionSet[DELEGATECALL].constantSmoke = params.CallSmokeEIP150
	return instructionSet
}

// newHomesteadInstructionSet returns the frontier and homestead
// instructions that can be executed during the homestead phase.
func newHomesteadInstructionSet() JumpTable {
	instructionSet := newFrontierInstructionSet()
	instructionSet[DELEGATECALL] = &operation{
		execute:     opDelegateCall,
		dynamicSmoke:  smokeDelegateCall,
		constantSmoke: params.CallSmokeFrontier,
		minStack:    minStack(6, 1),
		maxStack:    maxStack(6, 1),
		memorySize:  memoryDelegateCall,
		returns:     true,
	}
	return instructionSet
}

// newFrontierInstructionSet returns the frontier instructions
// that can be executed during the frontier phase.
func newFrontierInstructionSet() JumpTable {
	return JumpTable{
		STOP: {
			execute:     opStop,
			constantSmoke: 0,
			minStack:    minStack(0, 0),
			maxStack:    maxStack(0, 0),
			halts:       true,
		},
		ADD: {
			execute:     opAdd,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		MUL: {
			execute:     opMul,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SUB: {
			execute:     opSub,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		DIV: {
			execute:     opDiv,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SDIV: {
			execute:     opSdiv,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		MOD: {
			execute:     opMod,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SMOD: {
			execute:     opSmod,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		ADDMOD: {
			execute:     opAddmod,
			constantSmoke: SmokeMidStep,
			minStack:    minStack(3, 1),
			maxStack:    maxStack(3, 1),
		},
		MULMOD: {
			execute:     opMulmod,
			constantSmoke: SmokeMidStep,
			minStack:    minStack(3, 1),
			maxStack:    maxStack(3, 1),
		},
		EXP: {
			execute:    opExp,
			dynamicSmoke: smokeExpFrontier,
			minStack:   minStack(2, 1),
			maxStack:   maxStack(2, 1),
		},
		SIGNEXTEND: {
			execute:     opSignExtend,
			constantSmoke: SmokeFastStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		LT: {
			execute:     opLt,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		GT: {
			execute:     opGt,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SLT: {
			execute:     opSlt,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SGT: {
			execute:     opSgt,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		EQ: {
			execute:     opEq,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		ISZERO: {
			execute:     opIszero,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		AND: {
			execute:     opAnd,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		XOR: {
			execute:     opXor,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		OR: {
			execute:     opOr,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		NOT: {
			execute:     opNot,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		BYTE: {
			execute:     opByte,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
		},
		SHA3: {
			execute:     opSha3,
			constantSmoke: params.Sha3Smoke,
			dynamicSmoke:  smokeSha3,
			minStack:    minStack(2, 1),
			maxStack:    maxStack(2, 1),
			memorySize:  memorySha3,
		},
		ADDRESS: {
			execute:     opAddress,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		BALANCE: {
			execute:     opBalance,
			constantSmoke: params.BalanceSmokeFrontier,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		ORIGIN: {
			execute:     opOrigin,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		CALLER: {
			execute:     opCaller,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		CALLVALUE: {
			execute:     opCallValue,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		CALLDATALOAD: {
			execute:     opCallDataLoad,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		CALLDATASIZE: {
			execute:     opCallDataSize,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		CALLDATACOPY: {
			execute:     opCallDataCopy,
			constantSmoke: SmokeFastestStep,
			dynamicSmoke:  smokeCallDataCopy,
			minStack:    minStack(3, 0),
			maxStack:    maxStack(3, 0),
			memorySize:  memoryCallDataCopy,
		},
		CODESIZE: {
			execute:     opCodeSize,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		CODECOPY: {
			execute:     opCodeCopy,
			constantSmoke: SmokeFastestStep,
			dynamicSmoke:  smokeCodeCopy,
			minStack:    minStack(3, 0),
			maxStack:    maxStack(3, 0),
			memorySize:  memoryCodeCopy,
		},
		GASPRICE: {
			execute:     opSmokeprice,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		EXTCODESIZE: {
			execute:     opExtCodeSize,
			constantSmoke: params.ExtcodeSizeSmokeFrontier,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		EXTCODECOPY: {
			execute:     opExtCodeCopy,
			constantSmoke: params.ExtcodeCopyBaseFrontier,
			dynamicSmoke:  smokeExtCodeCopy,
			minStack:    minStack(4, 0),
			maxStack:    maxStack(4, 0),
			memorySize:  memoryExtCodeCopy,
		},
		BLOCKHASH: {
			execute:     opBlockhash,
			constantSmoke: SmokeExtStep,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		COINBASE: {
			execute:     opCoinbase,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		TIMESTAMP: {
			execute:     opTimestamp,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		NUMBER: {
			execute:     opNumber,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		DIFFICULTY: {
			execute:     opDifficulty,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		GASLIMIT: {
			execute:     opSmokeLimit,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		POP: {
			execute:     opPop,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(1, 0),
			maxStack:    maxStack(1, 0),
		},
		MLOAD: {
			execute:     opMload,
			constantSmoke: SmokeFastestStep,
			dynamicSmoke:  smokeMLoad,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
			memorySize:  memoryMLoad,
		},
		MSTORE: {
			execute:     opMstore,
			constantSmoke: SmokeFastestStep,
			dynamicSmoke:  smokeMStore,
			minStack:    minStack(2, 0),
			maxStack:    maxStack(2, 0),
			memorySize:  memoryMStore,
		},
		MSTORE8: {
			execute:     opMstore8,
			constantSmoke: SmokeFastestStep,
			dynamicSmoke:  smokeMStore8,
			memorySize:  memoryMStore8,
			minStack:    minStack(2, 0),
			maxStack:    maxStack(2, 0),
		},
		SLOAD: {
			execute:     opSload,
			constantSmoke: params.SloadSmokeFrontier,
			minStack:    minStack(1, 1),
			maxStack:    maxStack(1, 1),
		},
		SSTORE: {
			execute:    opSstore,
			dynamicSmoke: smokeSStore,
			minStack:   minStack(2, 0),
			maxStack:   maxStack(2, 0),
			writes:     true,
		},
		JUMP: {
			execute:     opJump,
			constantSmoke: SmokeMidStep,
			minStack:    minStack(1, 0),
			maxStack:    maxStack(1, 0),
			jumps:       true,
		},
		JUMPI: {
			execute:     opJumpi,
			constantSmoke: SmokeSlowStep,
			minStack:    minStack(2, 0),
			maxStack:    maxStack(2, 0),
			jumps:       true,
		},
		PC: {
			execute:     opPc,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		MSIZE: {
			execute:     opMsize,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		GAS: {
			execute:     opSmoke,
			constantSmoke: SmokeQuickStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		JUMPDEST: {
			execute:     opJumpdest,
			constantSmoke: params.JumpdestSmoke,
			minStack:    minStack(0, 0),
			maxStack:    maxStack(0, 0),
		},
		PUSH1: {
			execute:     opPush1,
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH2: {
			execute:     makePush(2, 2),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH3: {
			execute:     makePush(3, 3),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH4: {
			execute:     makePush(4, 4),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH5: {
			execute:     makePush(5, 5),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH6: {
			execute:     makePush(6, 6),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH7: {
			execute:     makePush(7, 7),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH8: {
			execute:     makePush(8, 8),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH9: {
			execute:     makePush(9, 9),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH10: {
			execute:     makePush(10, 10),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH11: {
			execute:     makePush(11, 11),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH12: {
			execute:     makePush(12, 12),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH13: {
			execute:     makePush(13, 13),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH14: {
			execute:     makePush(14, 14),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH15: {
			execute:     makePush(15, 15),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH16: {
			execute:     makePush(16, 16),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH17: {
			execute:     makePush(17, 17),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH18: {
			execute:     makePush(18, 18),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH19: {
			execute:     makePush(19, 19),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH20: {
			execute:     makePush(20, 20),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH21: {
			execute:     makePush(21, 21),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH22: {
			execute:     makePush(22, 22),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH23: {
			execute:     makePush(23, 23),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH24: {
			execute:     makePush(24, 24),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH25: {
			execute:     makePush(25, 25),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH26: {
			execute:     makePush(26, 26),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH27: {
			execute:     makePush(27, 27),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH28: {
			execute:     makePush(28, 28),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH29: {
			execute:     makePush(29, 29),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH30: {
			execute:     makePush(30, 30),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH31: {
			execute:     makePush(31, 31),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		PUSH32: {
			execute:     makePush(32, 32),
			constantSmoke: SmokeFastestStep,
			minStack:    minStack(0, 1),
			maxStack:    maxStack(0, 1),
		},
		DUP1: {
			execute:     makeDup(1),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(1),
			maxStack:    maxDupStack(1),
		},
		DUP2: {
			execute:     makeDup(2),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(2),
			maxStack:    maxDupStack(2),
		},
		DUP3: {
			execute:     makeDup(3),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(3),
			maxStack:    maxDupStack(3),
		},
		DUP4: {
			execute:     makeDup(4),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(4),
			maxStack:    maxDupStack(4),
		},
		DUP5: {
			execute:     makeDup(5),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(5),
			maxStack:    maxDupStack(5),
		},
		DUP6: {
			execute:     makeDup(6),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(6),
			maxStack:    maxDupStack(6),
		},
		DUP7: {
			execute:     makeDup(7),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(7),
			maxStack:    maxDupStack(7),
		},
		DUP8: {
			execute:     makeDup(8),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(8),
			maxStack:    maxDupStack(8),
		},
		DUP9: {
			execute:     makeDup(9),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(9),
			maxStack:    maxDupStack(9),
		},
		DUP10: {
			execute:     makeDup(10),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(10),
			maxStack:    maxDupStack(10),
		},
		DUP11: {
			execute:     makeDup(11),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(11),
			maxStack:    maxDupStack(11),
		},
		DUP12: {
			execute:     makeDup(12),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(12),
			maxStack:    maxDupStack(12),
		},
		DUP13: {
			execute:     makeDup(13),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(13),
			maxStack:    maxDupStack(13),
		},
		DUP14: {
			execute:     makeDup(14),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(14),
			maxStack:    maxDupStack(14),
		},
		DUP15: {
			execute:     makeDup(15),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(15),
			maxStack:    maxDupStack(15),
		},
		DUP16: {
			execute:     makeDup(16),
			constantSmoke: SmokeFastestStep,
			minStack:    minDupStack(16),
			maxStack:    maxDupStack(16),
		},
		SWAP1: {
			execute:     makeSwap(1),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(2),
			maxStack:    maxSwapStack(2),
		},
		SWAP2: {
			execute:     makeSwap(2),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(3),
			maxStack:    maxSwapStack(3),
		},
		SWAP3: {
			execute:     makeSwap(3),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(4),
			maxStack:    maxSwapStack(4),
		},
		SWAP4: {
			execute:     makeSwap(4),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(5),
			maxStack:    maxSwapStack(5),
		},
		SWAP5: {
			execute:     makeSwap(5),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(6),
			maxStack:    maxSwapStack(6),
		},
		SWAP6: {
			execute:     makeSwap(6),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(7),
			maxStack:    maxSwapStack(7),
		},
		SWAP7: {
			execute:     makeSwap(7),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(8),
			maxStack:    maxSwapStack(8),
		},
		SWAP8: {
			execute:     makeSwap(8),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(9),
			maxStack:    maxSwapStack(9),
		},
		SWAP9: {
			execute:     makeSwap(9),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(10),
			maxStack:    maxSwapStack(10),
		},
		SWAP10: {
			execute:     makeSwap(10),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(11),
			maxStack:    maxSwapStack(11),
		},
		SWAP11: {
			execute:     makeSwap(11),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(12),
			maxStack:    maxSwapStack(12),
		},
		SWAP12: {
			execute:     makeSwap(12),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(13),
			maxStack:    maxSwapStack(13),
		},
		SWAP13: {
			execute:     makeSwap(13),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(14),
			maxStack:    maxSwapStack(14),
		},
		SWAP14: {
			execute:     makeSwap(14),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(15),
			maxStack:    maxSwapStack(15),
		},
		SWAP15: {
			execute:     makeSwap(15),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(16),
			maxStack:    maxSwapStack(16),
		},
		SWAP16: {
			execute:     makeSwap(16),
			constantSmoke: SmokeFastestStep,
			minStack:    minSwapStack(17),
			maxStack:    maxSwapStack(17),
		},
		LOG0: {
			execute:    makeLog(0),
			dynamicSmoke: makeSmokeLog(0),
			minStack:   minStack(2, 0),
			maxStack:   maxStack(2, 0),
			memorySize: memoryLog,
			writes:     true,
		},
		LOG1: {
			execute:    makeLog(1),
			dynamicSmoke: makeSmokeLog(1),
			minStack:   minStack(3, 0),
			maxStack:   maxStack(3, 0),
			memorySize: memoryLog,
			writes:     true,
		},
		LOG2: {
			execute:    makeLog(2),
			dynamicSmoke: makeSmokeLog(2),
			minStack:   minStack(4, 0),
			maxStack:   maxStack(4, 0),
			memorySize: memoryLog,
			writes:     true,
		},
		LOG3: {
			execute:    makeLog(3),
			dynamicSmoke: makeSmokeLog(3),
			minStack:   minStack(5, 0),
			maxStack:   maxStack(5, 0),
			memorySize: memoryLog,
			writes:     true,
		},
		LOG4: {
			execute:    makeLog(4),
			dynamicSmoke: makeSmokeLog(4),
			minStack:   minStack(6, 0),
			maxStack:   maxStack(6, 0),
			memorySize: memoryLog,
			writes:     true,
		},
		CREATE: {
			execute:     opCreate,
			constantSmoke: params.CreateSmoke,
			dynamicSmoke:  smokeCreate,
			minStack:    minStack(3, 1),
			maxStack:    maxStack(3, 1),
			memorySize:  memoryCreate,
			writes:      true,
			returns:     true,
		},
		CALL: {
			execute:     opCall,
			constantSmoke: params.CallSmokeFrontier,
			dynamicSmoke:  smokeCall,
			minStack:    minStack(7, 1),
			maxStack:    maxStack(7, 1),
			memorySize:  memoryCall,
			returns:     true,
		},
		CALLCODE: {
			execute:     opCallCode,
			constantSmoke: params.CallSmokeFrontier,
			dynamicSmoke:  smokeCallCode,
			minStack:    minStack(7, 1),
			maxStack:    maxStack(7, 1),
			memorySize:  memoryCall,
			returns:     true,
		},
		RETURN: {
			execute:    opReturn,
			dynamicSmoke: smokeReturn,
			minStack:   minStack(2, 0),
			maxStack:   maxStack(2, 0),
			memorySize: memoryReturn,
			halts:      true,
		},
		SELFDESTRUCT: {
			execute:    opSuicide,
			dynamicSmoke: smokeSelfdestruct,
			minStack:   minStack(1, 0),
			maxStack:   maxStack(1, 0),
			halts:      true,
			writes:     true,
		},
	}
}
