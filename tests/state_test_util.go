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
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/420integrated/go-420coin/core/state/snapshot"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/hexutil"
	"github.com/420integrated/go-420coin/common/math"
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/core/rawdb"
	"github.com/420integrated/go-420coin/core/state"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/core/vm"
	"github.com/420integrated/go-420coin/crypto"
	"github.com/420integrated/go-420coin/420db"
	"github.com/420integrated/go-420coin/params"
	"github.com/420integrated/go-420coin/rlp"
	"golang.org/x/crypto/sha3"
)

// StateTest checks transaction processing without block context.
// See https://github.com/420coin/EIPs/issues/176 for the test format specification.
type StateTest struct {
	json stJSON
}

// StateSubtest selects a specific configuration of a General State Test.
type StateSubtest struct {
	Fork  string
	Index int
}

func (t *StateTest) UnmarshalJSON(in []byte) error {
	return json.Unmarshal(in, &t.json)
}

type stJSON struct {
	Env  stEnv                    `json:"env"`
	Pre  core.GenesisAlloc        `json:"pre"`
	Tx   stTransaction            `json:"transaction"`
	Out  hexutil.Bytes            `json:"out"`
	Post map[string][]stPostState `json:"post"`
}

type stPostState struct {
	Root    common.UnprefixedHash `json:"hash"`
	Logs    common.UnprefixedHash `json:"logs"`
	Indexes struct {
		Data  int `json:"data"`
		Smoke   int `json:"smoke"`
		Value int `json:"value"`
	}
}

//go:generate gencodec -type stEnv -field-override stEnvMarshaling -out gen_stenv.go

type stEnv struct {
	Coinbase   common.Address `json:"currentCoinbase"   gencodec:"required"`
	Difficulty *big.Int       `json:"currentDifficulty" gencodec:"required"`
	SmokeLimit   uint64         `json:"currentSmokeLimit"   gencodec:"required"`
	Number     uint64         `json:"currentNumber"     gencodec:"required"`
	Timestamp  uint64         `json:"currentTimestamp"  gencodec:"required"`
}

type stEnvMarshaling struct {
	Coinbase   common.UnprefixedAddress
	Difficulty *math.HexOrDecimal256
	SmokeLimit   math.HexOrDecimal64
	Number     math.HexOrDecimal64
	Timestamp  math.HexOrDecimal64
}

//go:generate gencodec -type stTransaction -field-override stTransactionMarshaling -out gen_sttransaction.go

type stTransaction struct {
	SmokePrice   *big.Int `json:"smokePrice"`
	Nonce      uint64   `json:"nonce"`
	To         string   `json:"to"`
	Data       []string `json:"data"`
	SmokeLimit   []uint64 `json:"smokeLimit"`
	Value      []string `json:"value"`
	PrivateKey []byte   `json:"secretKey"`
}

type stTransactionMarshaling struct {
	SmokePrice   *math.HexOrDecimal256
	Nonce      math.HexOrDecimal64
	SmokeLimit   []math.HexOrDecimal64
	PrivateKey hexutil.Bytes
}

// GetChainConfig takes a fork definition and returns a chain config.
// The fork definition can be
// - a plain forkname, e.g. `Byzantium`,
// - a fork basename, and a list of EIPs to enable; e.g. `Byzantium+1884+1283`.
func GetChainConfig(forkString string) (baseConfig *params.ChainConfig, eips []int, err error) {
	var (
		splitForks            = strings.Split(forkString, "+")
		ok                    bool
		baseName, eipsStrings = splitForks[0], splitForks[1:]
	)
	if baseConfig, ok = Forks[baseName]; !ok {
		return nil, nil, UnsupportedForkError{baseName}
	}
	for _, eip := range eipsStrings {
		if eipNum, err := strconv.Atoi(eip); err != nil {
			return nil, nil, fmt.Errorf("syntax error, invalid eip number %v", eipNum)
		} else {
			if !vm.ValidEip(eipNum) {
				return nil, nil, fmt.Errorf("syntax error, invalid eip number %v", eipNum)
			}
			eips = append(eips, eipNum)
		}
	}
	return baseConfig, eips, nil
}

// Subtests returns all valid subtests of the test.
func (t *StateTest) Subtests() []StateSubtest {
	var sub []StateSubtest
	for fork, pss := range t.json.Post {
		for i := range pss {
			sub = append(sub, StateSubtest{fork, i})
		}
	}
	return sub
}

// Run executes a specific subtest and verifies the post-state and logs
func (t *StateTest) Run(subtest StateSubtest, vmconfig vm.Config, snapshotter bool) (*snapshot.Tree, *state.StateDB, error) {
	snaps, statedb, root, err := t.RunNoVerify(subtest, vmconfig, snapshotter)
	if err != nil {
		return snaps, statedb, err
	}
	post := t.json.Post[subtest.Fork][subtest.Index]
	// N.B: We need to do this in a two-step process, because the first Commit takes care
	// of suicides, and we need to touch the coinbase _after_ it has potentially suicided.
	if root != common.Hash(post.Root) {
		return snaps, statedb, fmt.Errorf("post state root mismatch: got %x, want %x", root, post.Root)
	}
	if logs := rlpHash(statedb.Logs()); logs != common.Hash(post.Logs) {
		return snaps, statedb, fmt.Errorf("post state logs hash mismatch: got %x, want %x", logs, post.Logs)
	}
	return snaps, statedb, nil
}

// RunNoVerify runs a specific subtest and returns the statedb and post-state root
func (t *StateTest) RunNoVerify(subtest StateSubtest, vmconfig vm.Config, snapshotter bool) (*snapshot.Tree, *state.StateDB, common.Hash, error) {
	config, eips, err := GetChainConfig(subtest.Fork)
	if err != nil {
		return nil, nil, common.Hash{}, UnsupportedForkError{subtest.Fork}
	}
	vmconfig.ExtraEips = eips
	block := t.genesis(config).ToBlock(nil)
	snaps, statedb := MakePreState(rawdb.NewMemoryDatabase(), t.json.Pre, snapshotter)

	post := t.json.Post[subtest.Fork][subtest.Index]
	msg, err := t.json.Tx.toMessage(post)
	if err != nil {
		return nil, nil, common.Hash{}, err
	}
	context := core.NewEVMContext(msg, block.Header(), nil, &t.json.Env.Coinbase)
	context.GetHash = vmTestBlockHash
	evm := vm.NewEVM(context, statedb, config, vmconfig)
	
	if config.IsYoloV2(context.BlockNumber) {
		statedb.AddAddressToAccessList(msg.From())
		if dst := msg.To(); dst != nil {
			statedb.AddAddressToAccessList(*dst)
			// If it's a create-tx, the destination will be added inside evm.create
		}
		for _, addr := range evm.ActivePrecompiles() {
			statedb.AddAddressToAccessList(addr)
		}
	}
	smokepool := new(core.SmokePool)
	smokepool.AddSmoke(block.SmokeLimit())
	snapshot := statedb.Snapshot()
	if _, err := core.ApplyMessage(evm, msg, smokepool); err != nil {
		statedb.RevertToSnapshot(snapshot)
	}
	// Commit block
	statedb.Commit(config.IsEIP158(block.Number()))
	// Add 0-value mining reward. This only makes a difference in the cases
	// where
	// - the coinbase suicided, or
	// - there are only 'bad' transactions, which aren't executed. In those cases,
	//   the coinbase gets no txfee, so isn't created, and thus needs to be touched
	statedb.AddBalance(block.Coinbase(), new(big.Int))
	// And _now_ get the state root
	root := statedb.IntermediateRoot(config.IsEIP158(block.Number()))
	return snaps, statedb, root, nil
}

func (t *StateTest) smokeLimit(subtest StateSubtest) uint64 {
	return t.json.Tx.SmokeLimit[t.json.Post[subtest.Fork][subtest.Index].Indexes.Smoke]
}

func MakePreState(db fourtwentydb.Database, accounts core.GenesisAlloc, snapshotter bool) (*snapshot.Tree, *state.StateDB) {
	sdb := state.NewDatabase(db)
	statedb, _ := state.New(common.Hash{}, sdb, nil)
	for addr, a := range accounts {
		statedb.SetCode(addr, a.Code)
		statedb.SetNonce(addr, a.Nonce)
		statedb.SetBalance(addr, a.Balance)
		for k, v := range a.Storage {
			statedb.SetState(addr, k, v)
		}
	}
	// Commit and re-open to start with a clean state.
	root, _ := statedb.Commit(false)

	var snaps *snapshot.Tree
	if snapshotter {
		snaps = snapshot.New(db, sdb.TrieDB(), 1, root, false, false)
	}
	statedb, _ = state.New(root, sdb, snaps)
	return snaps, statedb
}

func (t *StateTest) genesis(config *params.ChainConfig) *core.Genesis {
	return &core.Genesis{
		Config:     config,
		Coinbase:   t.json.Env.Coinbase,
		Difficulty: t.json.Env.Difficulty,
		SmokeLimit:   t.json.Env.SmokeLimit,
		Number:     t.json.Env.Number,
		Timestamp:  t.json.Env.Timestamp,
		Alloc:      t.json.Pre,
	}
}

func (tx *stTransaction) toMessage(ps stPostState) (core.Message, error) {
	// Derive sender from private key if present.
	var from common.Address
	if len(tx.PrivateKey) > 0 {
		key, err := crypto.ToECDSA(tx.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %v", err)
		}
		from = crypto.PubkeyToAddress(key.PublicKey)
	}
	// Parse recipient if present.
	var to *common.Address
	if tx.To != "" {
		to = new(common.Address)
		if err := to.UnmarshalText([]byte(tx.To)); err != nil {
			return nil, fmt.Errorf("invalid to address: %v", err)
		}
	}

	// Get values specific to this post state.
	if ps.Indexes.Data > len(tx.Data) {
		return nil, fmt.Errorf("tx data index %d out of bounds", ps.Indexes.Data)
	}
	if ps.Indexes.Value > len(tx.Value) {
		return nil, fmt.Errorf("tx value index %d out of bounds", ps.Indexes.Value)
	}
	if ps.Indexes.Smoke > len(tx.SmokeLimit) {
		return nil, fmt.Errorf("tx smoke limit index %d out of bounds", ps.Indexes.Smoke)
	}
	dataHex := tx.Data[ps.Indexes.Data]
	valueHex := tx.Value[ps.Indexes.Value]
	smokeLimit := tx.SmokeLimit[ps.Indexes.Smoke]
	// Value, Data hex encoding is messy: https://github.com/420coin/tests/issues/203
	value := new(big.Int)
	if valueHex != "0x" {
		v, ok := math.ParseBig256(valueHex)
		if !ok {
			return nil, fmt.Errorf("invalid tx value %q", valueHex)
		}
		value = v
	}
	data, err := hex.DecodeString(strings.TrimPrefix(dataHex, "0x"))
	if err != nil {
		return nil, fmt.Errorf("invalid tx data %q", dataHex)
	}

	msg := types.NewMessage(from, to, tx.Nonce, value, smokeLimit, tx.SmokePrice, data, true)
	return msg, nil
}

func rlpHash(x interface{}) (h common.Hash) {
	hw := sha3.NewLegacyKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
