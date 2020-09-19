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

package params

import "math/big"

const (
	SmokeLimitBoundDivisor uint64 = 1024    // The bound divisor of the smoke limit, used in update calculations.
	MinSmokeLimit          uint64 = 5000    // Minimum the smoke limit may ever be.
	GenesisSmokeLimit      uint64 = 4712388 // Smoke limit of the Genesis block.

	MaximumExtraDataSize  uint64 = 32    // Maximum size extra data may be after Genesis.
	ExpByteSmoke            uint64 = 10    // Times ceil(log256(exponent)) for the EXP instruction.
	SloadSmoke              uint64 = 50    // Multiplied by the number of 32-byte words that are copied (round up) for any *COPY operation and added.
	CallValueTransferSmoke  uint64 = 9000  // Paid for CALL when the value transfer is non-zero.
	CallNewAccountSmoke     uint64 = 25000 // Paid for CALL when the destination address didn't exist prior.
	TxSmoke                 uint64 = 21000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxSmokeContractCreation uint64 = 53000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.
	TxDataZeroSmoke         uint64 = 4     // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions.
	QuadCoeffDiv          uint64 = 512   // Divisor for the quadratic particle of the memory cost equation.
	LogDataSmoke            uint64 = 8     // Per byte in a LOG* operation's data.
	CallStipend           uint64 = 2300  // Free smoke given at beginning of call.

	Sha3Smoke     uint64 = 30 // Once per SHA3 operation.
	Sha3WordSmoke uint64 = 6  // Once per word of the SHA3 operation's data.

	SstoreSetSmoke    uint64 = 20000 // Once per SLOAD operation.
	SstoreResetSmoke  uint64 = 5000  // Once per SSTORE operation if the zeroness changes from zero.
	SstoreClearSmoke  uint64 = 5000  // Once per SSTORE operation if the zeroness doesn't change.
	SstoreRefundSmoke uint64 = 15000 // Once per SSTORE operation if the zeroness changes to zero.

	NetSstoreNoopSmoke  uint64 = 200   // Once per SSTORE operation if the value doesn't change.
	NetSstoreInitSmoke  uint64 = 20000 // Once per SSTORE operation from clean zero.
	NetSstoreCleanSmoke uint64 = 5000  // Once per SSTORE operation from clean non-zero.
	NetSstoreDirtySmoke uint64 = 200   // Once per SSTORE operation from dirty.

	NetSstoreClearRefund      uint64 = 15000 // Once per SSTORE operation for clearing an originally existing storage slot
	NetSstoreResetRefund      uint64 = 4800  // Once per SSTORE operation for resetting to the original non-zero value
	NetSstoreResetClearRefund uint64 = 19800 // Once per SSTORE operation for resetting to the original zero value

	SstoreSentrySmokeEIP2200   uint64 = 2300  // Minimum smoke required to be present for an SSTORE call, not consumed
	SstoreNoopSmokeEIP2200     uint64 = 800   // Once per SSTORE operation if the value doesn't change.
	SstoreDirtySmokeEIP2200    uint64 = 800   // Once per SSTORE operation if a dirty value is changed.
	SstoreInitSmokeEIP2200     uint64 = 20000 // Once per SSTORE operation from clean zero to non-zero
	SstoreInitRefundEIP2200  uint64 = 19200 // Once per SSTORE operation for resetting to the original zero value
	SstoreCleanSmokeEIP2200    uint64 = 5000  // Once per SSTORE operation from clean non-zero to something else
	SstoreCleanRefundEIP2200 uint64 = 4200  // Once per SSTORE operation for resetting to the original non-zero value
	SstoreClearRefundEIP2200 uint64 = 15000 // Once per SSTORE operation for clearing an originally existing storage slot

	JumpdestSmoke   uint64 = 1     // Once per JUMPDEST operation.
	EpochDuration uint64 = 30000 // Duration between proof-of-work epochs.

	CreateDataSmoke            uint64 = 200   //
	CallCreateDepth          uint64 = 1024  // Maximum depth of call/create stack.
	ExpSmoke                   uint64 = 10    // Once per EXP instruction
	LogSmoke                   uint64 = 375   // Per LOG* operation.
	CopySmoke                  uint64 = 3     //
	StackLimit               uint64 = 1024  // Maximum size of VM stack allowed.
	TierStepSmoke              uint64 = 0     // Once per operation, for a selection of them.
	LogTopicSmoke              uint64 = 375   // Multiplied by the * of the LOG*, per LOG transaction. e.g. LOG0 incurs 0 * c_txLogTopicSmoke, LOG4 incurs 4 * c_txLogTopicSmoke.
	CreateSmoke                uint64 = 32000 // Once per CREATE operation & contract-creation transaction.
	Create2Smoke               uint64 = 32000 // Once per CREATE2 operation
	SelfdestructRefundSmoke    uint64 = 24000 // Refunded following a selfdestruct operation.
	MemorySmoke                uint64 = 3     // Times the address of the (highest referenced byte in memory + 1). NOTE: referencing happens on read, write and in instructions such as RETURN and CALL.
	TxDataNonZeroSmokeFrontier uint64 = 68    // Per byte of data attached to a transaction that is not equal to zero. NOTE: Not payable on data of calls between transactions.
	TxDataNonZeroSmokeEIP2028  uint64 = 16    // Per byte of non zero data attached to a transaction after EIP 2028 (part in Istanbul)

	// These have been changed during the course of the chain
	CallSmokeFrontier              uint64 = 40  // Once per CALL operation & message call transaction.
	CallSmokeEIP150                uint64 = 700 // Static portion of smoke for CALL-derivates after EIP 150 (Tangerine)
	BalanceSmokeFrontier           uint64 = 20  // The cost of a BALANCE operation
	BalanceSmokeEIP150             uint64 = 400 // The cost of a BALANCE operation after Tangerine
	BalanceSmokeEIP1884            uint64 = 700 // The cost of a BALANCE operation after EIP 1884 (part of Istanbul)
	ExtcodeSizeSmokeFrontier       uint64 = 20  // Cost of EXTCODESIZE before EIP 150 (Tangerine)
	ExtcodeSizeSmokeEIP150         uint64 = 700 // Cost of EXTCODESIZE after EIP 150 (Tangerine)
	SloadSmokeFrontier             uint64 = 50
	SloadSmokeEIP150               uint64 = 200
	SloadSmokeEIP1884              uint64 = 800  // Cost of SLOAD after EIP 1884 (part of Istanbul)
	SloadSmokeEIP2200              uint64 = 800  // Cost of SLOAD after EIP 2200 (part of Istanbul)
	ExtcodeHashSmokeConstantinople uint64 = 400  // Cost of EXTCODEHASH (introduced in Constantinople)
	ExtcodeHashSmokeEIP1884        uint64 = 700  // Cost of EXTCODEHASH after EIP 1884 (part in Istanbul)
	SelfdestructSmokeEIP150        uint64 = 5000 // Cost of SELFDESTRUCT post EIP 150 (Tangerine)

	// EXP has a dynamic portion depending on the size of the exponent
	ExpByteFrontier uint64 = 10 // was set to 10 in Frontier
	ExpByteEIP158   uint64 = 50 // was raised to 50 during Eip158 (Spurious Dragon)

	// Extcodecopy has a dynamic AND a static cost. This represents only the
	// static portion of the smoke. It was changed during EIP 150 (Tangerine)
	ExtcodeCopyBaseFrontier uint64 = 20
	ExtcodeCopyBaseEIP150   uint64 = 700

	// CreateBySelfdestructSmoke is used when the refunded account is one that does
	// not exist. This logic is similar to call.
	// Introduced in Tangerine Whistle (Eip 150)
	CreateBySelfdestructSmoke uint64 = 25000

	MaxCodeSize = 24576 // Maximum bytecode to permit for a contract

	// Precompiled contract smoke prices

	EcrecoverSmoke        uint64 = 3000 // Elliptic curve sender recovery smoke price
	Sha256BaseSmoke       uint64 = 60   // Base price for a SHA256 operation
	Sha256PerWordSmoke    uint64 = 12   // Per-word price for a SHA256 operation
	Ripemd160BaseSmoke    uint64 = 600  // Base price for a RIPEMD160 operation
	Ripemd160PerWordSmoke uint64 = 120  // Per-word price for a RIPEMD160 operation
	IdentityBaseSmoke     uint64 = 15   // Base price for a data copy operation
	IdentityPerWordSmoke  uint64 = 3    // Per-work price for a data copy operation
	ModExpQuadCoeffDiv  uint64 = 20   // Divisor for the quadratic particle of the big int modular exponentiation

	Bn256AddSmokeByzantium             uint64 = 500    // Byzantium smoke needed for an elliptic curve addition
	Bn256AddSmokeIstanbul              uint64 = 150    // Smoke needed for an elliptic curve addition
	Bn256ScalarMulSmokeByzantium       uint64 = 40000  // Byzantium smoke needed for an elliptic curve scalar multiplication
	Bn256ScalarMulSmokeIstanbul        uint64 = 6000   // Smoke needed for an elliptic curve scalar multiplication
	Bn256PairingBaseSmokeByzantium     uint64 = 100000 // Byzantium base price for an elliptic curve pairing check
	Bn256PairingBaseSmokeIstanbul      uint64 = 45000  // Base price for an elliptic curve pairing check
	Bn256PairingPerPointSmokeByzantium uint64 = 80000  // Byzantium per-point price for an elliptic curve pairing check
	Bn256PairingPerPointSmokeIstanbul  uint64 = 34000  // Per-point price for an elliptic curve pairing check

	Bls12381G1AddSmoke          uint64 = 600    // Price for BLS12-381 elliptic curve G1 point addition
	Bls12381G1MulSmoke          uint64 = 12000  // Price for BLS12-381 elliptic curve G1 point scalar multiplication
	Bls12381G2AddSmoke          uint64 = 4500   // Price for BLS12-381 elliptic curve G2 point addition
	Bls12381G2MulSmoke          uint64 = 55000  // Price for BLS12-381 elliptic curve G2 point scalar multiplication
	Bls12381PairingBaseSmoke    uint64 = 115000 // Base smoke price for BLS12-381 elliptic curve pairing check
	Bls12381PairingPerPairSmoke uint64 = 23000  // Per-point pair smoke price for BLS12-381 elliptic curve pairing check
	Bls12381MapG1Smoke          uint64 = 5500   // Smoke price for BLS12-381 mapping field element to G1 operation
	Bls12381MapG2Smoke          uint64 = 110000 // Smoke price for BLS12-381 mapping field element to G2 operation
)

// Smoke discount table for BLS12-381 G1 and G2 multi exponentiation operations
var Bls12381MultiExpDiscountTable = [128]uint64{1200, 888, 764, 641, 594, 547, 500, 453, 438, 423, 408, 394, 379, 364, 349, 334, 330, 326, 322, 318, 314, 310, 306, 302, 298, 294, 289, 285, 281, 277, 273, 269, 268, 266, 265, 263, 262, 260, 259, 257, 256, 254, 253, 251, 250, 248, 247, 245, 244, 242, 241, 239, 238, 236, 235, 233, 232, 231, 229, 228, 226, 225, 223, 222, 221, 220, 219, 219, 218, 217, 216, 216, 215, 214, 213, 213, 212, 211, 211, 210, 209, 208, 208, 207, 206, 205, 205, 204, 203, 202, 202, 201, 200, 199, 199, 198, 197, 196, 196, 195, 194, 193, 193, 192, 191, 191, 190, 189, 188, 188, 187, 186, 185, 185, 184, 183, 182, 182, 181, 180, 179, 179, 178, 177, 176, 176, 175, 174}

var (
	DifficultyBoundDivisor = big.NewInt(2048)   // The bound divisor of the difficulty, used in the update calculations.
	GenesisDifficulty      = big.NewInt(131072) // Difficulty of the Genesis block.
	MinimumDifficulty      = big.NewInt(131072) // The minimum that the difficulty may ever be.
	DurationLimit          = big.NewInt(13)     // The decision boundary on the blocktime duration used to determine if difficulty should go up or not.
)
