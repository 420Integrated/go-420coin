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

package fourtwenty

import (
	"context"
	"errors"
	"math/big"

	"github.com/420integrated/go-420coin/accounts"
	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/consensus"
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/core/bloombits"
	"github.com/420integrated/go-420coin/core/rawdb"
	"github.com/420integrated/go-420coin/core/state"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/core/vm"
	"github.com/420integrated/go-420coin/420/downloader"
	"github.com/420integrated/go-420coin/420/smokeprice"
	"github.com/420integrated/go-420coin/420db"
	"github.com/420integrated/go-420coin/event"
	"github.com/420integrated/go-420coin/miner"
	"github.com/420integrated/go-420coin/params"
	"github.com/420integrated/go-420coin/rpc"
)

// FourtwentyAPIBackend implements fourtwentyapi.Backend for full nodes
type FourtwentyAPIBackend struct {
	extRPCEnabled bool
	fourtwenty    *Fourtwentycoin
	gpo           *smokeprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *FourtwentyAPIBackend) ChainConfig() *params.ChainConfig {
	return b.fourtwenty.blockchain.Config()
}

func (b *FourtwentyAPIBackend) CurrentBlock() *types.Block {
	return b.fourtwenty.blockchain.CurrentBlock()
}

func (b *FourtwentyAPIBackend) SetHead(number uint64) {
	b.fourtwenty.protocolManager.downloader.Cancel()
	b.fourtwenty.blockchain.SetHead(number)
}

func (b *FourtwentyAPIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.fourtwenty.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.fourtwenty.blockchain.CurrentBlock().Header(), nil
	}
	return b.fourtwenty.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *FourtwentyAPIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.fourtwenty.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.fourtwenty.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *FourtwentyAPIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.fourtwenty.blockchain.GetHeaderByHash(hash), nil
}

func (b *FourtwentyAPIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.fourtwenty.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.fourtwenty.blockchain.CurrentBlock(), nil
	}
	return b.fourtwenty.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *FourtwentyAPIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.fourtwenty.blockchain.GetBlockByHash(hash), nil
}

func (b *FourtwentyAPIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.fourtwenty.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.fourtwenty.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.fourtwenty.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *FourtwentyAPIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, state := b.fourtwenty.miner.Pending()
		return state, block.Header(), nil
	}
	// Otherwise resolve the block number and return its state
	header, err := b.HeaderByNumber(ctx, number)
	if err != nil {
		return nil, nil, err
	}
	if header == nil {
		return nil, nil, errors.New("header not found")
	}
	stateDb, err := b.fourtwenty.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *FourtwentyAPIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.StateAndHeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header, err := b.HeaderByHash(ctx, hash)
		if err != nil {
			return nil, nil, err
		}
		if header == nil {
			return nil, nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.fourtwenty.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.fourtwenty.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *FourtwentyAPIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.fourtwenty.blockchain.GetReceiptsByHash(hash), nil
}

func (b *FourtwentyAPIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.fourtwenty.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *FourtwentyAPIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.fourtwenty.blockchain.GetTdByHash(hash)
}

func (b *FourtwentyAPIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	vmError := func() error { return nil }

	txContext := core.NewEVMTxContext(msg)
	context := core.NewEVMBlockContext(header, b.fourtwenty.BlockChain(), nil)
	return vm.NewEVM(context, txContext, state, b.fourtwenty.blockchain.Config(), *b.fourtwenty.blockchain.GetVMConfig()), vmError, nil
}

func (b *FourtwentyAPIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.fourtwenty.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *FourtwentyAPIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.fourtwenty.miner.SubscribePendingLogs(ch)
}

func (b *FourtwentyAPIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.fourtwenty.BlockChain().SubscribeChainEvent(ch)
}

func (b *FourtwentyAPIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.fourtwenty.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *FourtwentyAPIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.fourtwenty.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *FourtwentyAPIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.fourtwenty.BlockChain().SubscribeLogsEvent(ch)
}

func (b *FourtwentyAPIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.fourtwenty.txPool.AddLocal(signedTx)
}

func (b *FourtwentyAPIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.fourtwenty.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *FourtwentyAPIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.fourtwenty.txPool.Get(hash)
}

func (b *FourtwentyAPIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.fourtwenty.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *FourtwentyAPIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.fourtwenty.txPool.Nonce(addr), nil
}

func (b *FourtwentyAPIBackend) Stats() (pending int, queued int) {
	return b.fourtwenty.txPool.Stats()
}

func (b *FourtwentyAPIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.fourtwenty.TxPool().Content()
}

func (b *FourtwentyAPIBackend) TxPool() *core.TxPool {
	return b.fourtwenty.TxPool()
}

func (b *FourtwentyAPIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.fourtwenty.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *FourtwentyAPIBackend) Downloader() *downloader.Downloader {
	return b.fourtwenty.Downloader()
}

func (b *FourtwentyAPIBackend) ProtocolVersion() int {
	return b.fourtwenty.fourtwentyVersion()
}

func (b *FourtwentyAPIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *FourtwentyAPIBackend) ChainDb() fourtwentydb.Database {
	return b.fourtwenty.ChainDb()
}

func (b *FourtwentyAPIBackend) EventMux() *event.TypeMux {
	return b.fourtwenty.EventMux()
}

func (b *FourtwentyAPIBackend) AccountManager() *accounts.Manager {
	return b.fourtwenty.AccountManager()
}

func (b *FourtwentyAPIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *FourtwentyAPIBackend) RPCSmokeCap() uint64 {
	return b.fourtwenty.config.RPCSmokeCap
}

func (b *FourtwentyAPIBackend) RPCTxFeeCap() float64 {
	return b.fourtwenty.config.RPCTxFeeCap
}

func (b *FourtwentyAPIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.fourtwenty.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *FourtwentyAPIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.fourtwenty.bloomRequests)
	}
}

func (b *FourtwentyAPIBackend) Engine() consensus.Engine {
	return b.fourtwenty.engine
}

func (b *FourtwentyAPIBackend) CurrentHeader() *types.Header {
	return b.fourtwenty.blockchain.CurrentHeader()
}

func (b *FourtwentyAPIBackend) Miner() *miner.Miner {
	return b.fourtwenty.Miner()
}

func (b *FourtwentyAPIBackend) StartMining(threads int) error {
	return b.fourtwenty.StartMining(threads)
}
