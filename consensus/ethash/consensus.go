// Copyright 2020 420Integrated
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
Coin Distribution (Block #1-#1,111,110)
Miners: 87%
Veterans Fund: 13%
Coin Distribution (attaching Cannasseur Network "followers' rewards") (Block #1,111,111-#2,102,399)
Miners: 80%
Veterans Fund: 13%
Followers: 7%
Coin distribution (Final Fork) (Block #2,102.400 onwards)
Miners: 75%
Veterans Fund: 15%
Followers: 10%
*/
// Ethash proof-of-work protocol constants.
var (
    finalBlockReward *big.Int = big.NewInt(9e+18) // Generalized block reward, in marleys. (9.0 420coins)
    slowBlockReward *big.Int  = big.NewInt(3e+18) // Slow-start block reward, in marleys, during blockchain intiation
    maxUncles                 = 2 // Maximum number of uncles allowed in a single block
    SlowStart *big.Int             = big.NewInt(10000) // Slow-start duration. Approx 40 hrs.
    rewardBlockDivisor *big.Int    = big.NewInt(100000)
    rewardBlockFlat *big.Int       = big.NewInt(1000000)

    rewardDistMinerPre *big.Int    = big.NewInt(87) // 87% of 9 420coins (7.83 420coin)
    rewardDistMinerPost *big.Int   = big.NewInt(80) // 80% of 9 420coins (7.20 420coin)
    rewardDistCannasseurBlock *big.Int = big.NewInt(1111111) 
    rewardDistFollower *big.Int    = big.NewInt(7)
    rewardDistVet *big.Int         = big.NewInt(13)
    forkBlock *big.Int             = big.NewInt(1111111)
		// Final Reward status
		finalForkBlock *big.Int = big.NewInt(2102400) // 
		fFrewardDistFollower *big.Int = big.NewInt(15)
		fFrewardDistVet *big.Int = big.NewInt(10)
		fFrewardDistMiner *big.Int = big.NewInt(75)

	)

// Various error messages to mark blocks invalid. 
var (
	errLargeBlockTime    = errors.New("timestamp too big")
	errZeroBlockTime     = errors.New("timestamp equals parent's")
	errTooManyUncles     = errors.New("too many uncles")
	errDuplicateUncle    = errors.New("duplicate uncle")
	errUncleIsAncestor   = errors.New("uncle is ancestor")
	errDanglingUncle     = errors.New("uncle's parent is not ancestor")
	errNonceOutOfRange   = errors.New("nonce out of range")
	errInvalidDifficulty = errors.New("non-positive difficulty")
	errInvalidMixDigest  = errors.New("invalid mix digest")
	errInvalidPoW        = errors.New("invalid proof-of-work")
)

// function Author implements consensus.Engine, returning the header coinbase as the
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
// concurrently. 
func (ethash *Ethash) VerifyHeaders(chain consensus.ChainReader, headers []*types.Header, seals []bool) (chan<- struct{}, <-chan error) {
	if ethash.fakeFull || len(headers) == 0 {
		abort, results := make(chan struct{}), make(chan error, len(headers))
		for i := 0; i < len(headers); i++ {
			results <- nil
		}
		return abort, results
	}

	workers := runtime.GOMAXPROCS(0)
	if len(headers) < workers {
		workers = len(headers)
	}

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

func (ethash *Ethash) verifyHeaderWorker(chain consensus.ChainReader, headers []*types.Header, seals []bool, index int) error {
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
		return nil 
	}
	return ethash.verifyHeader(chain, headers[index], parent, false, seals[index])
}

// VerifyUncles verifies that the given block's uncles conform to the consensus
// rules of the 420coin ethash consensus engine.
func (ethash *Ethash) VerifyUncles(chain consensus.ChainReader, block *types.Block) error {
	if ethash.fakeFull {
		return nil
	}
	// Verify that there are at most 2 uncles included in this block
	if len(block.Uncles()) > maxUncles {
		return errTooManyUncles
	}
	// Gather the set of past uncles and ancestors
	uncles, ancestors := set.New(), make(map[common.Hash]*types.Header)

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
		if uncles.Has(hash) {
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
// 420coin ethash consensus engine.
func (ethash *Ethash) verifyHeader(chain consensus.ChainReader, header, parent *types.Header, uncle bool, seal bool) error {
	// Ensure that the header's extra-data section is of a reasonable size
	if uint64(len(header.Extra)) > params.MaximumExtraDataSize {
		return fmt.Errorf("extra-data too long: %d > %d", len(header.Extra), params.MaximumExtraDataSize)
	}
	// Verify the header's timestamp
	if uncle {
		if header.Time.Cmp(math.MaxBig256) > 0 {
			return errLargeBlockTime
		}
	} else {
		if header.Time.Cmp(big.NewInt(time.Now().Unix())) > 0 {
			return consensus.ErrFutureBlock
		}
	}
	if header.Time.Cmp(parent.Time) <= 0 {
		return errZeroBlockTime
	}
	// Verify the block's difficulty based in it's timestamp and parent's difficulty
	expected := CalcDifficulty(chain.Config(), header.Time.Uint64(), parent)
	
	if expected.Cmp(header.Difficulty) != 0 {
		return fmt.Errorf("invalid difficulty: have %v, want %v", header.Difficulty, expected)
	}
	// Verify that the smoke limit is <= 2^63-1
	cap := uint64(0x7fffffffffffffff)
	if header.SmokeLimit > cap {
		return fmt.Errorf("invalid smokeLimit: have %v, max %v", header.SmokeLimit, math.MaxBig63)
	}
	// Verify that the smokeUsed is <= smokeLimit
	if header.SmokeUsed.Cmp(header.SmokeLimit) > 0 {
		return fmt.Errorf("invalid smokeUsed: have %v, smokeLimit %v", header.SmokeUsed, header.SmokeLimit)
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

func CalcDifficulty(config *params.ChainConfig, time uint64, parent *types.Header) *big.Int {
	next := new(big.Int).Add(parent.Number, common.Big1)
	switch {
	case config.IsHomestead(next):
		return calcDifficultyFourtwenty(time, parent)
	default:
		return calcDifficultyFourtwenty(time, parent)
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
// the difficulty is calculated with Byzantium rules.
func makeDifficultyCalculator(bombDelay *big.Int) func(time uint64, parent *types.Header) *big.Int {
	bombDelayFromParent := new(big.Int).Sub(bombDelay, big1)
	return func(time uint64, parent *types.Header) *big.Int {
		// algorithm:
		// diff = (parent_diff +
		//         (parent_diff / 2048 * max((2 if len(parent.uncles) else 1) - ((timestamp - parent.timestamp) // 9), -99))
		//        ) + 2^(periodCount - 2)

		bigTime := new(big.Int).SetUint64(time)
		bigParentTime := new(big.Int).SetUint64(parent.Time)
		
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
		fakeBlockNumber := new(big.Int)
		if parent.Number.Cmp(bombDelayFromParent) >= 0 {
			fakeBlockNumber = fakeBlockNumber.Sub(parent.Number, bombDelayFromParent)
		}
		periodCount := fakeBlockNumber
		periodCount.Div(periodCount, expDiffPeriod)

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
// parent block's time and difficulty. 
func calcDifficultyHomestead(time uint64, parent *types.Header) *big.Int {
	// algorithm:
	// diff = (parent_diff +
	//         (parent_diff / 2048 * max(1 - (block_timestamp - parent_timestamp) // 10, -99))
	//        ) + 2^(periodCount - 2)

	bigTime := new(big.Int).SetUint64(time)
	bigParentTime := new(big.Int).Set(parent.Time)

	x := new(big.Int)
	y := new(big.Int)

	// 1 - (block_timestamp -parent_timestamp) // 10
	x.Sub(bigTime, bigParentTime)
	x.Div(x, big10)
	x.Sub(common.Big1, x)

	// max(1 - (block_timestamp - parent_timestamp) // 10, -99)))
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

	periodCount := new(big.Int).Add(parent.Number, common.Big1)
	periodCount.Div(periodCount, expDiffPeriod)

	// diff = diff + 2^(periodCount - 2)
	if periodCount.Cmp(common.Big1) > 0 {
		y.Sub(periodCount, common.Big2)
		y.Exp(common.Big2, y, nil)
		x.Add(x, y)
	}
	return x
}

// calcDifficultyFrontier is the difficulty adjustment algorithm. It returns the
// difficulty that a new block should have when created at time given the parent
// block's time and difficulty. The calculation uses the Frontier rules.
func calcDifficultyWhale(time uint64, parent *types.Header) *big.Int {
	diff := new(big.Int)
	adjust := new(big.Int).Div(parent.Difficulty, big10)
	bigTime := new(big.Int)
	bigParentTime := new(big.Int)

	bigTime.SetUint64(time)
	bigParentTime.Set(parent.Time)

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
	// If we're running a shared PoW, delegate verification to it
	if ethash.shared != nil {
		return ethash.shared.VerifySeal(chain, header, fulldag)
	}
	// Ensure that we have a valid difficulty for the block
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

			runtime.KeepAlive(dataset)
		} else {
			fulldag = false
		}
	}
	// If slow-but-light PoW verification was requested (or DAG has not completed download), use an ethash cache
	if !fulldag {
		cache := ethash.cache(number)

		size := datasetSize(number)
		if ethash.config.PowMode == ModeTest {
			size = 32 * 1024
		}
		digest, result = hashimotoLight(size, cache.cache, ethash.SealHash(header).Bytes(), header.Nonce.Uint64())

		runtime.KeepAlive(cache)
	}
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
func (ethash *Ethash) Prepare(chain consensus.ChainReader, header *types.Header) error {
	parent := chain.GetHeader(header.ParentHash, header.Number.Uint64()-1)
	if parent == nil {
		return consensus.ErrUnknownAncestor
	}
	header.Difficulty = CalcDifficulty(chain, header.Time, parent)
	return nil
}

// Finalize implements consensus.Engine, accumulating the block and uncle rewards,
// setting the final state and assembling the block.
func (ethash *Ethash) Finalize(chain consensus.ChainHeaderReader, header *types.Header, state *state.StateDB, txs []*types.Transaction, uncles []*types.Header, receipts []*types.Receipt) (*types.Block, error) {
	vaultState := chain.GetHeaderByNumber(0)
	AccumulateNewRewards(state, header, uncles, vaultState)
	// AccumulateRewards(state, header, uncles)
	header.Root = state.IntermediateRoot(chain.Config().IsEIP158(header.Number))

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
// TODO (karalabe): Move the chain maker into this package and make this private!
// func AccumulateRewards(state *state.StateDB, header *types.Header, uncles []*types.Header) {
// 	reward := new(big.Int).Set(blockReward)
// 	r := new(big.Int)
// 	for _, uncle := range uncles {
// 		r.Add(uncle.Number, big8)
// 		r.Sub(r, header.Number)
// 		r.Mul(r, blockReward)
// 		r.Div(r, big8)
// 		state.AddBalance(uncle.Coinbase, r)

// 		r.Div(blockReward, big32)
// 		reward.Add(reward, r)
// 	}
// 	state.AddBalance(header.Coinbase, reward)
// }

func AccumulateNewRewards(config *params.ChainConfig, state *state.StateDB, header *types.Header, uncles []*types.Header, genesisHeader *types.Header) {
	creatorAddress := common.BytesToAddress(genesisHeader.Extra)
	contractAddress := crypto.CreateAddress(creatorAddress, 0)
	changeAtBlock := state.GetState(contractAddress, common.BytesToHash([]byte{0})).Big()
	var devRewardAddress common.Address
	var followerRewardAddress common.Address
	if (header.Number.Cmp(changeAtBlock) == 1) {
		devAddrBytes := state.GetState(contractAddress, common.BytesToHash([]byte{1})).Bytes()
		devRewardAddress = common.BytesToAddress(devAddrBytes[len(devAddrBytes)-20:])
		followerAddrBytes := state.GetState(contractAddress, common.BytesToHash([]byte{2})).Bytes()
		followerRewardAddress = common.BytesToAddress(followerAddrBytes[len(followerAddrBytes)-20:])
	} else {
		devAddrBytesprev := state.GetState(contractAddress, common.BytesToHash([]byte{3})).Bytes()
		devRewardAddress = common.BytesToAddress(devAddrBytesprev[len(devAddrBytesprev)-20:])
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
    reward := new(big.Int)
    headerRew := new(big.Int)
    headerRew.Div(header.Number, rewardBlockDivisor)
    if (header.Number.Cmp(SlowStart)  == -1 || header.Number.Cmp(SlowStart)  == 0) {
        reward = reward.Set(slowBlockReward)
    } else if (header.Number.Cmp(rewardBlockFlat) == 1) {
        reward = reward.Set(finalBlockReward)
    } else {
    	headerRew.Mul(headerRew, slowBlockReward)
        reward = reward.Sub(initialBlockReward, headerRew)
    }
    //fmt.Println(header.Number, reward)
    r := new(big.Int)
    minerReward := new(big.Int)
    contractReward :=new(big.Int)
    contractRewardSplit := new(big.Int)
		fFVetReward := new(big.Int)
		fFFollowerReward := new(big.Int)
    cumulativeReward := new(big.Int)
    rewardDivisor := big.NewInt(100)
    // if block.Number > 200000
    if (header.Number.Cmp(rewardDistCannasseurBlock) == 1) {
			if (header.Number.Cmp(founderForkBlock) == 1) {
				for _, uncle := range uncles {
		        r.Add(uncle.Number, big8)
		        r.Sub(r, header.Number)
		        r.Mul(r, reward)
		        r.Div(r, big8)
		  		// calcuting miner reward Post Final Fork Block
		        minerReward.Mul(r, fFrewardDistMiner)
		        minerReward.Div(minerReward, rewardDivisor)
		                // calculating rewards to be sent to Veterans Fund contract Post Final Fork
				fFVetReward.Mul(r, fFrewardDistVet)
				fFVetReward.Div(fFVetReward, rewardDivisor)
				// Calculating "followers" rewards to be sent to the Cannasseur Network contract post Final Fork
				fFFollowerReward.Mul(r, fFrewardDistFollower)
				fFFollowerReward.Div(fFFollowerReward, rewardDivisor)

		        state.AddBalance(uncle.Coinbase, minerReward)
		        state.AddBalance(devRewardAddress, fFVetReward)
		        state.AddBalance(followerRewardAddress, fFFollowerReward)
		        r.Div(reward, big32)
		        reward.Add(reward, r)
		    }
				// calcuting miner reward Post Switch Block
				// calcuting miner reward Post FinalFork Block
					minerReward.Mul(reward, fFrewardDistMiner)
					minerReward.Div(minerReward, rewardDivisor)
					// calculating rewards to be sent to Veterans Fund contract Post Final Fork
					fFVetReward.Mul(reward, fFrewardDistVet)
					fFVetReward.Div(fFVetReward, rewardDivisor)
					// Calculating follower rewards to be sent to the contract post Final Fork
					fFFollowerReward.Mul(reward, fFrewardDistFollower)
					fFFollowerReward.Div(fFFollowerReward, rewardDivisor)

		      state.AddBalance(vetRewardAddress, fFVetReward)
		      state.AddBalance(followerRewardAddress, fFFollowerReward)
		      state.AddBalance(header.Coinbase, minerReward)
			} else {

    	for _, uncle := range uncles {
	        r.Add(uncle.Number, big8)
	        r.Sub(r, header.Number)
	        r.Mul(r, reward)
	        r.Div(r, big8)
	  		// calcuting miner reward Post Switch Block
	        minerReward.Mul(r, rewardDistMinerPost)
	        minerReward.Div(minerReward, rewardDivisor)
	        // calculating cumulative rewards to be sent to contract Post Switch block
	        cumulativeReward.Add(rewardDistFollower, rewardDistDev) //per 100
	        // Calculating contract reward Post Switch Block
	        contractReward.Mul(r, cumulativeReward)
	        contractReward.Div(contractReward, rewardDivisor)

	        state.AddBalance(uncle.Coinbase, minerReward)
	        contractRewardSplit.Div(contractReward, big.NewInt(2))
	        state.AddBalance(vetRewardAddress, contractRewardSplit)
	        state.AddBalance(followerRewardAddress, contractRewardSplit)
	        r.Div(reward, big32)
	        reward.Add(reward, r)
	    }
  	    // calcuting miner reward Post Switch Block
	    minerReward.Mul(reward, rewardDistMinerPost)
	    minerReward.Div(minerReward, rewardDivisor)
	    // calculating cumulative rewards to be sent to contract Post Switch block
	    cumulativeReward.Add(rewardDistFollower, rewardDistVet) //per 100
	    // Calculating contract reward Post Switch Block
	    contractReward.Mul(reward, cumulativeReward)
	    contractReward.Div(contractReward, rewardDivisor)

        contractRewardSplit.Div(contractReward, big.NewInt(2))
        state.AddBalance(vetRewardAddress, contractRewardSplit)
        state.AddBalance(followerRewardAddress, contractRewardSplit)
        if (header.Number.Cmp(forkBlock) == 1) {
         	state.AddBalance(header.Coinbase, minerReward)
        }
	//fmt.Println(state.GetBalance(header.Coinbase), state.GetBalance(devRewardAddress), state.GetBalance(followerRewardAddress))
	}} else {
		for _, uncle := range uncles {
	        r.Add(uncle.Number, big8)
	        r.Sub(r, header.Number)
	        r.Mul(r, reward)
	        r.Div(r, big8)
	  	// calcuting miner reward Pre Switch Block
	        minerReward.Mul(r, rewardDistMinerPre)
	        minerReward.Div(minerReward, rewardDivisor)
	        // calculating rewards to be sent to Veterans Fund contract prior to Switch
	        contractReward.Mul(r, rewardDistVet)
	        contractReward.Div(contractReward, rewardDivisor)

	        state.AddBalance(uncle.Coinbase, minerReward)
	        state.AddBalance(vetRewardAddress, contractReward)
	        r.Div(reward, big32)
	        reward.Add(reward, r)
	    }
	    // calcuting miner reward Pre Switch Block
	    minerReward.Mul(reward, rewardDistMinerPre)
	    minerReward.Div(minerReward, rewardDivisor)
	    // Calculating reward proportion for Veterans Fund during prior to Switch Block
	    contractReward.Mul(reward, rewardDistVet)
	    contractReward.Div(contractReward, rewardDivisor)

	    state.AddBalance(vetRewardAddress, contractReward)
	    state.AddBalance(header.Coinbase, minerReward)
	   // fmt.Println(state.GetBalance(header.Coinbase), state.GetBalance(devRewardAddress))
	}
}
