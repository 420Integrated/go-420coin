// Code generated by github.com/fjl/gencodec. DO NOT EDIT.

package core

import (
	"encoding/json"
	"errors"
	"math/big"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/hexutil"
	"github.com/420integrated/go-420coin/common/math"
	"github.com/420integrated/go-420coin/params"
)

var _ = (*genesisSpecMarshaling)(nil)

func (g Genesis) MarshalJSON() ([]byte, error) {
	type Genesis struct {
		Config     *params.ChainConfig                         `json:"config"`
		Nonce      math.HexOrDecimal64                         `json:"nonce"`
		Timestamp  math.HexOrDecimal64                         `json:"timestamp"`
		ExtraData  hexutil.Bytes                               `json:"extraData"`
		SmokeLimit   math.HexOrDecimal64                         `json:"smokeLimit"   gencodec:"required"`
		Difficulty *math.HexOrDecimal256                       `json:"difficulty" gencodec:"required"`
		Mixhash    common.Hash                                 `json:"mixHash"`
		Coinbase   common.Address                              `json:"coinbase"`
		Alloc      map[common.UnprefixedAddress]GenesisAccount `json:"alloc"      gencodec:"required"`
		Number     math.HexOrDecimal64                         `json:"number"`
		SmokeUsed    math.HexOrDecimal64                         `json:"smokeUsed"`
		ParentHash common.Hash                                 `json:"parentHash"`
	}
	var enc Genesis
	enc.Config = g.Config
	enc.Nonce = math.HexOrDecimal64(g.Nonce)
	enc.Timestamp = math.HexOrDecimal64(g.Timestamp)
	enc.ExtraData = g.ExtraData
	enc.SmokeLimit = math.HexOrDecimal64(g.SmokeLimit)
	enc.Difficulty = (*math.HexOrDecimal256)(g.Difficulty)
	enc.Mixhash = g.Mixhash
	enc.Coinbase = g.Coinbase
	if g.Alloc != nil {
		enc.Alloc = make(map[common.UnprefixedAddress]GenesisAccount, len(g.Alloc))
		for k, v := range g.Alloc {
			enc.Alloc[common.UnprefixedAddress(k)] = v
		}
	}
	enc.Number = math.HexOrDecimal64(g.Number)
	enc.SmokeUsed = math.HexOrDecimal64(g.SmokeUsed)
	enc.ParentHash = g.ParentHash
	return json.Marshal(&enc)
}

func (g *Genesis) UnmarshalJSON(input []byte) error {
	type Genesis struct {
		Config     *params.ChainConfig                         `json:"config"`
		Nonce      *math.HexOrDecimal64                        `json:"nonce"`
		Timestamp  *math.HexOrDecimal64                        `json:"timestamp"`
		ExtraData  *hexutil.Bytes                              `json:"extraData"`
		SmokeLimit   *math.HexOrDecimal64                        `json:"smokeLimit"   gencodec:"required"`
		Difficulty *math.HexOrDecimal256                       `json:"difficulty" gencodec:"required"`
		Mixhash    *common.Hash                                `json:"mixHash"`
		Coinbase   *common.Address                             `json:"coinbase"`
		Alloc      map[common.UnprefixedAddress]GenesisAccount `json:"alloc"      gencodec:"required"`
		Number     *math.HexOrDecimal64                        `json:"number"`
		SmokeUsed    *math.HexOrDecimal64                        `json:"smokeUsed"`
		ParentHash *common.Hash                                `json:"parentHash"`
	}
	var dec Genesis
	if err := json.Unmarshal(input, &dec); err != nil {
		return err
	}
	if dec.Config != nil {
		g.Config = dec.Config
	}
	if dec.Nonce != nil {
		g.Nonce = uint64(*dec.Nonce)
	}
	if dec.Timestamp != nil {
		g.Timestamp = uint64(*dec.Timestamp)
	}
	if dec.ExtraData != nil {
		g.ExtraData = *dec.ExtraData
	}
	if dec.SmokeLimit == nil {
		return errors.New("missing required field 'smokeLimit' for Genesis")
	}
	g.SmokeLimit = uint64(*dec.SmokeLimit)
	if dec.Difficulty == nil {
		return errors.New("missing required field 'difficulty' for Genesis")
	}
	g.Difficulty = (*big.Int)(dec.Difficulty)
	if dec.Mixhash != nil {
		g.Mixhash = *dec.Mixhash
	}
	if dec.Coinbase != nil {
		g.Coinbase = *dec.Coinbase
	}
	if dec.Alloc == nil {
		return errors.New("missing required field 'alloc' for Genesis")
	}
	g.Alloc = make(GenesisAlloc, len(dec.Alloc))
	for k, v := range dec.Alloc {
		g.Alloc[common.Address(k)] = v
	}
	if dec.Number != nil {
		g.Number = uint64(*dec.Number)
	}
	if dec.SmokeUsed != nil {
		g.SmokeUsed = uint64(*dec.SmokeUsed)
	}
	if dec.ParentHash != nil {
		g.ParentHash = *dec.ParentHash
	}
	return nil
}
