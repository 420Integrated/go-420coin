package core

import (
	"fmt"
	"math/big"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/hexutil"
	"github.com/420integrated/go-420coin/common/math"
)

// GnosisSafeTx is a type to parse the safe-tx returned by the relayer,
// it also conforms to the API required by the Gnosis Safe tx relay service.
// See 'SafeMultisigTransaction' on https://safe-transaction.mainnet.gnosis.io/
type GnosisSafeTx struct {
	// These fields are only used on output
	Signature  hexutil.Bytes           `json:"signature"`
	SafeTxHash common.Hash             `json:"contractTransactionHash"`
	Sender     common.MixedcaseAddress `json:"sender"`
	// These fields are used both on input and output
	Safe           common.MixedcaseAddress `json:"safe"`
	To             common.MixedcaseAddress `json:"to"`
	Value          math.Decimal256         `json:"value"`
	SmokePrice       math.Decimal256       `json:"smokePrice"`
	Data           *hexutil.Bytes          `json:"data"`
	Operation      uint8                   `json:"operation"`
	SmokeToken       common.Address        `json:"smokeToken"`
	RefundReceiver common.Address          `json:"refundReceiver"`
	BaseSmoke        big.Int               `json:"baseSmoke"`
	SafeTxSmoke      big.Int               `json:"safeTxSmoke"`
	Nonce          big.Int                 `json:"nonce"`
	InputExpHash   common.Hash             `json:"safeTxHash"`
}

// ToTypedData converts the tx to a EIP-712 Typed Data structure for signing
func (tx *GnosisSafeTx) ToTypedData() TypedData {
	var data hexutil.Bytes
	if tx.Data != nil {
		data = *tx.Data
	}
	gnosisTypedData := TypedData{
		Types: Types{
			"EIP712Domain": []Type{{Name: "verifyingContract", Type: "address"}},
			"SafeTx": []Type{
				{Name: "to", Type: "address"},
				{Name: "value", Type: "uint256"},
				{Name: "data", Type: "bytes"},
				{Name: "operation", Type: "uint8"},
				{Name: "safeTxSmoke", Type: "uint256"},
				{Name: "baseSmoke", Type: "uint256"},
				{Name: "smokePrice", Type: "uint256"},
				{Name: "smokeToken", Type: "address"},
				{Name: "refundReceiver", Type: "address"},
				{Name: "nonce", Type: "uint256"},
			},
		},
		Domain: TypedDataDomain{
			VerifyingContract: tx.Safe.Address().Hex(),
		},
		PrimaryType: "SafeTx",
		Message: TypedDataMessage{
			"to":             tx.To.Address().Hex(),
			"value":          tx.Value.String(),
			"data":           data,
			"operation":      fmt.Sprintf("%d", tx.Operation),
			"safeTxSmoke":    fmt.Sprintf("%#d", &tx.SafeTxSmoke),
			"baseSmoke":      fmt.Sprintf("%#d", &tx.BaseSmoke),
			"smokePrice":     tx.SmokePrice.String(),
			"smokeToken":     tx.SmokeToken.Hex(),
			"refundReceiver": tx.RefundReceiver.Hex(),
			"nonce":          fmt.Sprintf("%d", tx.Nonce.Uint64()),
		},
	}
	return gnosisTypedData
}

// ArgsForValidation returns a SendTxArgs struct, which can be used for the
// common validations, e.g. look up 4byte destinations
func (tx *GnosisSafeTx) ArgsForValidation() *SendTxArgs {
	args := &SendTxArgs{
		From:       tx.Safe,
		To:         &tx.To,
		Smoke:      hexutil.Uint64(tx.SafeTxSmoke.Uint64()),
		SmokePrice: hexutil.Big(tx.SmokePrice),
		Value:      hexutil.Big(tx.Value),
		Nonce:      hexutil.Uint64(tx.Nonce.Uint64()),
		Data:       tx.Data,
		Input:      nil,
	}
	return args
}
