// Copyright 202 420Inetgrated
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

package ethash

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"
	"runtime"
	"time"

	mapset "github.com/deckarep/golang-set"
	"github.com/420integrated/go-420coin/crypto"
	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/math"
	"github.com/420integrated/go-420coin/consensus"
	"github.com/420integrated/go-420coin/consensus/misc"
	"github.com/420integrated/go-420coin/core/state"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/params"
	"github.com/420integrated/go-420coin/rlp"
	"github.com/420integrated/go-420coin/trie"
	"golang.org/x/crypto/sha3"
)

/*
Coin Distribution Ruderalis Era (Block #1-#1,111,110) (Slow start blockReward first 1000 blocks)
Miners: 87%
Veterans Fund: 13%
Coin Distribution Indica Era (attaching Cannasseur Network "followers' rewards") (Block #1,111,111-#2,102,399)
initiating approximately six months from Genesis
Miners: 80%
Veterans Fund: 13%
Followers: 7%
Coin distribution Sativa Era (Block #2,102.400 onwards)
initiating approximately 1 year from Genesis
Miners: 75%
Veterans Fund: 15%
Followers: 10%
*/

// Ethash proof-of-work protocol constants.
var (
	SativaBlockReward *big.Int  = big.NewInt(9e+18)  // Generalized block reward, in marleys. (9.0 420coins)
        slowBlockReward *big.Int    = big.NewInt(3e+18)  // Slow-start block reward, in marleys, during blockchain intiation
	maxUncles                   = 2                 // Maximum number of uncles allowed in a single block
	SlowStart *big.Int          = big.NewInt(1000)
	allowedFutureBlockTime      = 15 * time.Second  // Max time from current time allowed for blocks, before they're considered future blocks
  
        rewardBlockDivisor *big.Int    = big.NewInt(100000)
        rewardBlockFlat *big.Int       = big.NewInt(1000000)
        // Reward split between Miners, Veterans Fund and, following Cannasseur Network initiation, "Followers"
        rewardDistMinerRuderalis *big.Int  = big.NewInt(87) // 87% of 9 420coins (7.83 420coin)
        rewardDistMinerIndica *big.Int     = big.NewInt(80) // 80% of 9 420coins (7.20 420coin)
        rewardDistCannasseurBlock *big.Int = big.NewInt(1111111) // approximately 6 months following Genesis
	rewardDistFollower *big.Int    = big.NewInt(7) // 7% of 9 420coin (0.63 420coin)
	rewardDistVet *big.Int         = big.NewInt(13) // 13% of 9 420coin (1.17 420coin)
        indicaForkBlock *big.Int       = big.NewInt(1111111)
	// Final Reward status
	sativaForkBlock *big.Int          = big.NewInt(2102400) // approximately 1 year following Genesis
	sativaRewardDistFollower *big.Int = big.NewInt(15) // 15% of block reward - 1.35 420coins
	sativaRewardDistVet *big.Int      = big.NewInt(10) // 10% of block reward - 0.9 420coins
	sativaRewardDistMiner *big.Int    = big.NewInt(75) // 75% of block reward - 6.75 420coins
	
	// calcDifficultyEip2384 is the difficulty adjustment algorithm as specified by EIP 2384.
	// It offsets the bomb 4M blocks from Constantinople, so in total 9M blocks.
	// Specification EIP-2384: https://eips.ethereum.org/EIPS/eip-2384
	calcDifficultyEip2384 = makeDifficultyCalculator(big.NewInt(9000000))

	// calcDifficultyConstantinople is the difficulty adjustment algorithm for Constantinople.
	// It returns the difficulty that a new block should have when created at time given the
	// parent block's time and difficulty. The calculation uses the Byzantium rules, but with
	// bomb offset 5M.
	// Specification EIP-1234: https://eips.ethereum.org/EIPS/eip-1234
	calcDifficultyConstantinople = makeDifficultyCalculator(big.NewInt(5000000))

	// calcDifficultyByzantium is the difficulty adjustment algorithm. It returns
	// the difficulty that a new block should have when created at time given the
	// parent block's time and difficulty. The calculation uses the Byzantium rules.
	// Specification EIP-649: https://eips.ethereum.org/EIPS/eip-649
	calcDifficultyByzantium = makeDifficultyCalculator(big.NewInt(3000000))
)

// Various error messages to mark blocks invalid. These should be private to
// prevent engine specific errors from being referenced in the remainder of the
// codebase, inherently breaking if the engine is swapped out. Please put common
// error types into the consensus package.
var (
	errOlderBlockTime    = errors.New("timestamp older than parent")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errNonceOutOfRange   = errors.New("nonce out of range")
)

// Author implements consensus.Engine, returning the header's coinbase as the
// proof-of-work verified author of the block.
func (ethash *Ethash) Author(header *types.Header) (common.Address, error) {
	return header.Coinbase, nil
}

// VerifyHeader checks whether a header conforms to the consensus rules of the
// 420coin ethash engine.
func (ethash *Ethash) VerifyHeader(chain consensus.ChainHeaderReader, header *types.Header, seal bool) error {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	// Short circuit if the header is known, or its parent not
	number := header.Number.Uint64()
	if chain.GetHeader(header.Hash(), number) != nil {
		return nil
	}
	parent := chain.GetHeader(header.ParentHash, number-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	// Sanity checks passed, do a proper verification
	return ethash.verifyHeader(chain, header, parent, false, seal)
}

// VerifyHeaders is similar to VerifyHeader, but verifies a batch of headers
// concurrently. The method returns a quit channel to abort the operations and
// a results channel to retrieve the async verifications.
func (ethash *Ethash) VerifyHeaders(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	// Spawn as many workers as allowed threads
	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

	// Create a task channel and spawn the verifiers
	var (
		inputs = make(chan int)
		done   = make(chan int, workers)
		errors = make([]error, len(headers))
		abort  = make(chan struct{})
	)
	for i := 0; i < workers; i++ {
		go func() {
			for index := range inputs {
				errors[index] = ethash.verifyHeaderWorker(chain, headers, seals, index)
				done <- index
			}
		}()
	}

	errorsOut := make(chan error, len(headers))
	go func() {
		defer close(inputs)
		var (
			in, out = 0, 0
			checked = make([]bool, len(headers))
			inputs  = inputs
		)
		for {
			select {
			case inputs <- in:
				if in++; in == len(headers) {
					// Reached end of headers. Stop sending to workers.
					inputs = nil
				}
			case index := <-done:
				for checked[index] = true; checked[out]; out++ {
					errorsOut <- errors[out]
					if out == len(headers)-1 {
						return
					}
				}
			case <-abort:
				return
			}
		}
	}()
	return abort, errorsOut
}

func (ethash *Ethash) verifyHeaderWorker(chain consensus.ChainHeaderReader, headers []*types.Header, seals []bool, index int) error {
	var parent *types.Header
	if index == 0 {
		parent = chain.GetHeader(headers[0].ParentHash, headers[0].Number.Uint64()-1)
	} else if headers[index-1].Hash() == headers[index].ParentHash {
		parent = headers[index-1]
	}
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	if chain.GetHeader(headers[index].Hash(), headers[index].Number.Uint64()) != nil {
		return nil // known block
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index])
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the stock 420coin ethash engine.
func (ethash *Ethash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	// If we're running a full engine faking, accept any input as valid
	if ethash.config.PowMode == ModeFullFake {
		return nil
	}
	// Verify that there are at most 2 uncles included in this block
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
	if len(block.Uncles()) == 0 {
		return nil
	}
	// Gather the set of past uncles and ancestors
	uncles, ancestors := mapset.NewSet(), make(map[common.Hash]*types.Header)

	number, parent := block.NumberU64()-1, block.ParentHash()
	for i := 0; i < 7; i++ {
		ancestor := chain.GetBlock(parent, number)
		if ancestor == nil {
			break
		}
		ancestors[ancestor.Hash()] = ancestor.Header()
		for _, uncle := range ancestor.Uncles() {
			uncles.Add(uncle.Hash())
		}
		parent, number = ancestor.ParentHash(), number-1
	}
	ancestors[block.Hash()] = block.Header()
	uncles.Add(block.Hash())

	// Verify each of the uncles that it's recent, but not an ancestor
	for _, uncle := range block.Uncles() {
		// Make sure every uncle is rewarded only once
		hash := uncle.Hash()
		if uncles.Contains(hash) {
			return errDuplicateUncle
		}
		uncles.Add(hash)

		// Make sure the uncle has a valid ancestry
		if ancestors[hash] != nil {
			return errUncleIsAncestor
		}
		if ancestors[uncle.ParentHash] == nil || uncle.ParentHash == block.ParentHash() {
			return errDanglingUncle
		}
		if err := ethash.verifyHeader(chain, uncle, ancestors[uncle.ParentHash], true, true); err != nil {
			return err
		}
	}
	return nil
}

// verifyHeader checks whether a header conforms to the consensus rules of the
// 420coin ethash engine.
func (ethash *Ethash) verifyHeader(chain consensus.ChainHeaderReader, header, parent *types.Header, uncle bool, seal bool) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if !uncle {
		if header.Time > uint64(time.Now().Add(allowedFutureBlockTime).Unix()) {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time <= parent.Time {
		return errOlderBlockTime
	}
	// Verify the block's difficulty based on its timestamp and parent's difficulty
	expected := ethash.CalcDifficulty(chain, header.Time, parent)

	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
	// Verify that the smoke limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.SmokeLimit > cap {
		return fmt.Errorf("invalid smokeLimit: have %v, max %v", header.SmokeLimit, cap)
	}
	// Verify that the smokeUsed is <= smokeLimit
	if header.SmokeUsed > header.SmokeLimit {
		return fmt.Errorf("invalid smokeUsed: have %d, smokeLimit %d", header.SmokeUsed, header.SmokeLimit)
	}
	// Verify that the smoke limit remains within allowed bounds
	diff := int64(parent.SmokeLimit) - int64(header.SmokeLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parent.SmokeLimit / params.SmokeLimitBoundDivisor

	if uint64(diff) >= limit || header.SmokeLimit < params.MinSmokeLimit {
		return fmt.Errorf("invalid smoke limit: have %d, want %d += %d", header.SmokeLimit, parent.SmokeLimit, limit)
	}
        // Verify that the block number is parent's +1
	if diff := new(big.Int).Sub(header.Number, parent.Number); diff.Cmp(big.NewInt(1)) != 0 {
		return consensus.ErrInvalidNumber
	}
	// Verify the engine specific seal securing the block
	if seal {
		if err := ethash.VerifySeal(chain, header); err != nil {
			return err
		}
	}
	// If all checks passed, validate any special fields for hard forks
	if err := misc.VerifyDAOHeaderExtraData(chain.Config(), header); err != nil {
		return err
	}
	if err := misc.VerifyForkHashes(chain.Config(), header, uncle); err != nil {
		return err
	}
	return nil
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func (ethash *Ethash) CalcDifficulty(chain consensus.ChainHeaderReader, time uint64, parent *types.Header) *big.Int {
	return CalcDifficulty(chain.Config(), time, parent)
}

// CalcDifficulty is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time
// given the parent block's time and difficulty.
func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	next := new(big.Int).Add(parent.Number, big1)
	switch {
	case config.IsMuirGlacier(next):
		return calcDifficultyEip2384(time, parent)
	case config.IsConstantinople(next):
		return calcDifficultyConstantinople(time, parent)
	case config.IsByzantium(next):
		return calcDifficultyByzantium(time, parent)
	case config.IsHomestead(next):
		return calcDifficultyHomestead(time, parent)
	default:
		return calcDifficultyFrontier(time, parent)
	}
}

// Some weird constants to avoid constant memory allocs for them.
var (
	expDiffPeriod = big.NewInt(100000)
	big1          = big.NewInt(1)
	big2          = big.NewInt(2)
	big9          = big.NewInt(9)
	big10         = big.NewInt(10)
	bigMinus99    = big.NewInt(-99)
)

// makeDifficultyCalculator creates a difficultyCalculator with the given bomb-delay.
// the difficulty is calculated with Byzantium rules, which differs from Homestead in
// how uncles affect the calculation
func makeDifficultyCalculator(bombDelay *big.Int) func(time uint64, parent *types.Header) *big.Int {
	// Note, the calculations below looks at the parent number, which is 1 below
	// the block number. Thus we remove one from the delay given
	bombDelayFromParent := new(big.Int).Sub(bombDelay, big1)
	return func(time uint64, parent *types.Header) *big.Int {
		// algorithm:
		// diff = (parent_diff +
		//         (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
		//        ) + 2^(periodCount - 2)

		bigTime := new(big.Int).SetUint64(time)
		bigParentTime := new(big.Int).SetUint64(parent.Time)

		// holds intermediate values to make the algo easier to read & audit
		x := new(big.Int)
		y := new(big.Int)

		// (2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9
		x.Sub(bigTime, bigParentTime)
		x.Div(x, big9)
		if parent.UncleHash == types.EmptyUncleHash {
			x.Sub(big1, x)
		} else {
			x.Sub(big2, x)
		}
		// max((2 if len(parent_uncles) else 1) - (block_timestamp - parent_timestamp) // 9, -99)
		if x.Cmp(bigMinus99) < 0 {
			x.Set(bigMinus99)
		}
		// parent_diff + (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
		y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
		x.Mul(y, x)
		x.Add(parent.Difficulty, x)

		// minimum difficulty can ever be (before exponential factor)
		if x.Cmp(params.MinimumDifficulty) < 0 {
			x.Set(params.MinimumDifficulty)
		}
		// calculate a fake block number for the ice-age delay
		// Specification: https://eips.ethereum.org/EIPS/eip-1234
		fakeBlockNumber := new(big.Int)
		if parent.Number.Cmp(bombDelayFromParent) >= 0 {
			fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, bombDelayFromParent)
		}
		// for the exponential factor
		periodCount := fakeBlockNumber
		periodCount.Div(periodCount, expDiffPeriod)

		// the exponential factor, commonly referred to as "the bomb"
		// diff = diff + 2^(periodCount - 2)
		if periodCount.Cmp(big1) > 0 {
			y.Sub(periodCount, big2)
			y.Exp(big2, y, nil)
			x.Add(x, y)
		}
		return x
	}
}

// calcDifficultyHomestead is the difficulty adjustment algorithm. It returns
// the difficulty that a new block should have when created at time given the
// parent block's time and difficulty. The calculation uses the Homestead rules.
func calcDifficultyHomestead(time uint64, parent *types.Header) *big.Int {
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).SetUint64(parent.Time)

	// holds intermediate values to make the algo easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	// 1 - (block_timestamp - parent_timestamp) // 10
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(big1, x)

	// max(1 - (block_timestamp - parent_timestamp) // 10, -99)
	if x.Cmp(bigMinus99) < 0 {
		x.Set(bigMinus99)
	}
	// (parent_diff + parent_diff // 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	y.Div(parent.Difficulty, params.DifficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parent.Difficulty, x)

	// minimum difficulty can ever be (before exponential factor)
	if x.Cmp(params.MinimumDifficulty) < 0 {
		x.Set(params.MinimumDifficulty)
	}
	// for the exponential factor
	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)

	// the exponential factor, commonly referred to as "the bomb"
	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(big1) > 0 {
		y.Sub(periodCount, big2)
		y.Exp(big2, y, nil)
		x.Add(x, y)
	}
	return x
}

// calcDifficultyFrontier is the difficulty adjustment algorithm. It returns the
// difficulty that a new block should have when created at time given the parent
// block's time and difficulty. The calculation uses the Frontier rules.
func calcDifficultyFrontier(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, params.DifficultyBoundDivisor)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.SetUint64(parent.Time)

	if bigTime.Sub(bigTime, bigParentTime).Cmp(params.DurationLimit) < 0 {
		diff.Add(parent.Difficulty, adjust)
	} else {
		diff.Sub(parent.Difficulty, adjust)
	}
	if diff.Cmp(params.MinimumDifficulty) < 0 {
		diff.Set(params.MinimumDifficulty)
	}

	periodCount := new(big.Int).Add(parent.Number, big1)
	periodCount.Div(periodCount, expDiffPeriod)
	if periodCount.Cmp(big1) > 0 {
		// diff = diff + 2^(periodCount - 2)
		expDiff := periodCount.Sub(periodCount, big2)
		expDiff.Exp(big2, expDiff, nil)
		diff.Add(diff, expDiff)
		diff = math.BigMax(diff, params.MinimumDifficulty)
	}
	return diff
}

// Exported for fuzzing
var FrontierDifficultyCalulator = calcDifficultyFrontier
var HomesteadDifficultyCalulator = calcDifficultyHomestead
var DynamicDifficultyCalculator = makeDifficultyCalculator

// VerifySeal implements consensus.Engine, checking whether the given block satisfies
// the PoW difficulty requirements.
func (ethash *Ethash) VerifySeal(chain consensus.ChainHeaderReader, header *types.Header) error {
	return ethash.verifySeal(chain, header, false)
}

// verifySeal checks whether a block satisfies the PoW difficulty requirements,
// either using the usual ethash cache for it, or alternatively using a full DAG
// to make remote mining fast.
func (ethash *Ethash) verifySeal(chain consensus.ChainHeaderReader, header *types.Header, fulldag bool) error {
	if ethash.config.PowMode == ModeFake || ethash.config.PowMode == ModeFullFake {
		time.Sleep(ethash.fakeDelay)
		if ethash.fakeFail == header.Number.Uint64() {
			return errInvalidPoW
		}
		return nil
	}
	// If running a shared PoW, delegate verification to it
	if ethash.shared != nil {
		return ethash.shared.verifySeal(chain, header, fulldag)
	}
	// Ensure valid difficulty for the block
	if header.Difficulty.Sign() <= 0 {
		return errInvalidDifficulty
	}
	// Recompute the digest and PoW values
	number := header.Number.Uint64()

	var (
		digest []byte
		result []byte
	)
	// If fast-but-heavy PoW verification was requested, use an ethash dataset
	if fulldag {
		dataset := ethash.dataset(number, true)
		if dataset.generated() {
			digest, result = hashimotoFull(dataset.dataset, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

			// Datasets are unmapped in a finalizer. Ensure that the dataset stays alive
			// until after the call to hashimotoFull so it's not unmapped while being used.
			runtime.KeepAlive(dataset)
		} else {
			// Dataset not yet generated, don't hang, use a cache instead
			fulldag = false
		}
	}
	// If slow-but-light PoW verification was requested (or DAG not yet ready), use an ethash cache
	if !fulldag {
		cache := ethash.cache(number)

		size := datasetSize(number)
		if ethash.config.PowMode == ModeTest {
			size = 32 * 1024
		}
		digest, result = hashimotoLight(size, cache.cache, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

		// Caches are unmapped in a finalizer. Ensure that the cache stays alive
		// until after the call to hashimotoLight so it's not unmapped while being used.
		runtime.KeepAlive(cache)
	}
	// Verify the calculated values against the ones provided in the header
	if !bytes.Equal(header.MixDigest[:], digest) {
		return errInvalidMixDigest
	}
	target := new(big.Int).Div(two256, header.Difficulty)
	if new(big.Int).SetBytes(result).Cmp(target) > 0 {
		return errInvalidPoW
	}
	return nil
}

// Prepare implements consensus.Engine, initializing the difficulty field of a
// header to conform to the ethash protocol. The changes are done inline.
func (ethash *Ethash) Prepare(chain consensus.ChainHeaderReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = ethash.CalcDifficulty(chain, header.Time, parent)
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state on the header
func (ethash *Ethash) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header) {
	// Accumulate block and uncle rewards then commit the final state root
	vaultState := chain.GetHeaderByNumber(0)
	AccumulateNewRewards(chain.Config(), state, header, uncles, vaultState)
	// Header complete, assemble into a block and return
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))
}

// FinalizeAndAssemble implements consensus.Engine, accumulating the block and
// uncle rewards, setting the final state and assembling the block.
func (ethash *Ethash) FinalizeAndAssemble(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	vaultState := chain.GetHeaderByNumber(0)
	// Finalize block
	ethash.Finalize(chain, header, state, txs, uncles)

	// Header seems complete, assemble into a block and return
	return types.NewBlock(header, txs, uncles, receipts, new(trie.Trie)), nil
}

// SealHash returns the hash of a block prior to it being sealed.
func (ethash *Ethash) SealHash(header *types.Header) (hash common.Hash) {
	hasher := sha3.NewLegacyKeccak256()

	rlp.Encode(hasher, []interface{}{
		header.ParentHash,
		header.UncleHash,
		header.Coinbase,
		header.Root,
		header.TxHash,
		header.ReceiptHash,
		header.Bloom,
		header.Difficulty,
		header.Number,
		header.SmokeLimit,
		header.SmokeUsed,
		header.Time,
		header.Extra,
	})
	hasher.Sum(hash[:0])
	return hash
}

// Some weird constants to avoid constant memory allocs for them.
var (
	big8  = big.NewInt(8)
	big32 = big.NewInt(32)
)

// AccumulateRewards credits the coinbase of the given block with the mining
// reward. The total reward consists of the static block reward and rewards for
// included uncles. The coinbase of each uncle block is also rewarded.
func AccumulateNewRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header, genesisHeader *types.Header) {
	// Select the correct block reward and proportion of reward to parties based on chain progression
	creatorAddress := common.BytesToAddress(genesisHeader.Extra)
	contractAddress := crypto.CreateAddress(creatorAddress, 0)
	changeAtBlock := state.GetState(contractAddress, common.BytesToHash([]byte{0})).Big()
	var vetRewardAddress common.Address
	var followerRewardAddress common.Address
	if (header.Number.Cmp(changeAtBlock) == 1) {
		vetAddrBytes := state.GetState(contractAddress, common.BytesToHash([]byte{1})).Bytes()
		vetRewardAddress = common.BytesToAddress(vetAddrBytes[len(vetAddrBytes)-20:])
		followerAddrBytes := state.GetState(contractAddress, common.BytesToHash([]byte{2})).Bytes()
		followerRewardAddress = common.BytesToAddress(followerAddrBytes[len(followerAddrBytes)-20:])
	} else {
		vetAddrBytesprev := state.GetState(contractAddress, common.BytesToHash([]byte{3})).Bytes()
		vetRewardAddress = common.BytesToAddress(vetAddrBytesprev[len(vetAddrBytesprev)-20:])
		followerAddrBytesprev := state.GetState(contractAddress, common.BytesToHash([]byte{4})).Bytes()
		followerRewardAddress = common.BytesToAddress(followerAddrBytesprev[len(followerAddrBytesprev)-20:])
	}
	//fmt.Println(header.Number, "header Number")
	//fmt.Println(changeAtBlock, "changeAtBlock")
	//fmt.Println(devRewardAddress.Hex(), "devRewardAddress")
	//fmt.Println(followerRewardAddress.Hex(), "followerRewardAddress")
	//fmt.Println("###################################################")

        initialBlockReward := new(big.Int)
        initialBlockReward.SetString("9000000000000000000",10)	
	// Accumulate the rewards for the miner and any included uncles
	reward := new(big.Int)
	headerRew := new(big.Int)
        headerRew.Div(header.Number, rewardBlockDivisor)
        if (header.Number.Cmp(SlowStart)  == -1 || header.Number.Cmp(SlowStart)  == 0) {
            reward = reward.Set(slowBlockReward)
        } else if (header.Number.Cmp(rewardBlockFlat) == 1) {
            reward = reward.Set(SativaBlockReward)
        } else {
    	    headerRew.Mul(headerRew, slowBlockReward)
            reward = reward.Sub(initialBlockReward, headerRew)
    }
	//fmt.Println(header.Number, reward)
	r := new(big.Int)
	minerReward := new(big.Int)
        contractReward :=new(big.Int)
        contractRewardSplit := new(big.Int)
		sativaVetReward := new(big.Int)
		sativaFollowerReward := new(big.Int)
        cumulativeReward := new(big.Int)
        rewardDivisor := big.NewInt(100)
        // if block.Number > 1111111
        if (header.Number.Cmp(rewardDistCannasseurBlock) == 1) {
		if (header.Number.Cmp(sativaForkBlock) == 1) {
	for _, uncle := range uncles {
		r.Add(uncle.Number, big8)
		r.Sub(r, header.Number)
		r.Mul(r, reward)
		r.Div(r, big8)
		        // calcuting miner reward Post Sativa Fork Block
		        minerReward.Mul(r, sativaRewardDistMiner)
		        minerReward.Div(minerReward, rewardDivisor)
		        // calculating rewards to be sent to Veterans Fund contract Post Sativa Fork
			sativaVetReward.Mul(r, sativaRewardDistVet)
			sativaVetReward.Div(sativaVetReward, rewardDivisor)
			// Calculating "followers" rewards to be sent to the Cannasseur Network contract post Sativa Fork
			sativaFollowerReward.Mul(r, sativaRewardDistFollower)
			sativaFollowerReward.Div(sativaFollowerReward, rewardDivisor)
		state.AddBalance(uncle.Coinbase, minerReward)
		state.AddBalance(vetRewardAddress, sativaVetReward)
		state.AddBalance(followerRewardAddress, sativaFollowerReward)
		r.Div(reward, big32)
		reward.Add(reward, r)
	}
                        // calcuting miner reward Post Indica Block
                        // calcuting miner reward Post Sativa Fork Block
	                minerReward.Mul(reward, sativaRewardDistMiner)
	                minerReward.Div(minerReward, rewardDivisor)
		        // calculating rewards to be sent to Veterans Fund contract Post Sativa Fork
		        sativaVetReward.Mul(reward, sativaRewardDistVet)
		        sativaVetReward.Div(sativaVetReward, rewardDivisor)
		        // Calculating follower rewards to be sent to the contract post Sativa Fork
		        sativaFollowerReward.Mul(reward, sativaRewardDistFollower)
		        sativaFollowerReward.Div(sativaFollowerReward, rewardDivisor)

		state.AddBalance(vetRewardAddress, sativaVetReward)
		state.AddBalance(followerRewardAddress, sativaFollowerReward)
		state.AddBalance(header.Coinbase, minerReward)
			} else {

    	for _, uncle := range uncles {
	        r.Add(uncle.Number, big8)
	        r.Sub(r, header.Number)
	        r.Mul(r, reward)
	        r.Div(r, big8)
		
	  	        // calcuting miner reward Post Indica Block
	                minerReward.Mul(r, rewardDistMinerIndica)
	                minerReward.Div(minerReward, rewardDivisor)
	                // calculating cumulative rewards to be sent to Cannasseur Network contract Post Indica block
	                cumulativeReward.Add(rewardDistFollower, rewardDistVet) 
	                // Calculating contract reward Post Indica Block
	                contractReward.Mul(r, cumulativeReward)
	                contractReward.Div(contractReward, rewardDivisor)

	        state.AddBalance(uncle.Coinbase, minerReward)
	        contractRewardSplit.Div(contractReward, big.NewInt(2))
	        state.AddBalance(vetRewardAddress, contractRewardSplit)
	        state.AddBalance(followerRewardAddress, contractRewardSplit)
	        r.Div(reward, big32)
	        reward.Add(reward, r)
	    }
  		        // calcuting miner reward Post Indica Block
	                minerReward.Mul(reward, rewardDistMinerIndica)
	                minerReward.Div(minerReward, rewardDivisor)
	                // calculating cumulative rewards to be sent to contract Post Indica block
	                cumulativeReward.Add(rewardDistFollower, rewardDistVet) //per 100
	                // Calculating contract reward Post Indica Block
	                contractReward.Mul(reward, cumulativeReward)
	                contractReward.Div(contractReward, rewardDivisor)

                contractRewardSplit.Div(contractReward, big.NewInt(2))
                state.AddBalance(vetRewardAddress, contractRewardSplit)
                state.AddBalance(followerRewardAddress, contractRewardSplit)
                if (header.Number.Cmp(indicaForkBlock) == 1) {
         	state.AddBalance(header.Coinbase, minerReward)
        }
	    //fmt.Println(state.GetBalance(header.Coinbase), state.GetBalance(devRewardAddress), state.GetBalance(followerRewardAddress))
	}} else {
		for _, uncle := range uncles {
	        r.Add(uncle.Number, big8)
	        r.Sub(r, header.Number)
	        r.Mul(r, reward)
	        r.Div(r, big8)
	  	// calcuting miner reward Pre Indica Block
	        minerReward.Mul(r, rewardDistMinerRuderalis)
	        minerReward.Div(minerReward, rewardDivisor)
	        // Calculating reward for Veterans Fund contract Pre Indica Block
	        contractReward.Mul(r, rewardDistVet)
	        contractReward.Div(contractReward, rewardDivisor)

	        state.AddBalance(uncle.Coinbase, minerReward)
	        state.AddBalance(vetRewardAddress, contractReward)
	        r.Div(reward, big32)
	        reward.Add(reward, r)
	    }
		// calcuting miner reward Pre Indica Block
	        minerReward.Mul(reward, rewardDistMinerRuderalis)
	        minerReward.Div(minerReward, rewardDivisor)
	        // Calculating Dev reward Pre Indica Block
	        contractReward.Mul(reward, rewardDistVet)
	        contractReward.Div(contractReward, rewardDivisor)

	        state.AddBalance(vetRewardAddress, contractReward)
	        state.AddBalance(header.Coinbase, minerReward)
	        // fmt.Println(state.GetBalance(header.Coinbase), state.GetBalance(vetRewardAddress))
	}
}
