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

package 420

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

// 420APIBackend implements 420api.Backend for full nodes
type 420APIBackend struct {
	extRPCEnabled bool
	420           *420coin
	gpo           *smokeprice.Oracle
}

// ChainConfig returns the active chain configuration.
func (b *420APIBackend) ChainConfig() *params.ChainConfig {
	return b.420.blockchain.Config()
}

func (b *420APIBackend) CurrentBlock() *types.Block {
	return b.420.blockchain.CurrentBlock()
}

func (b *420APIBackend) SetHead(number uint64) {
	b.420.protocolManager.downloader.Cancel()
	b.420.blockchain.SetHead(number)
}

func (b *420APIBackend) HeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Header, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.420.miner.PendingBlock()
		return block.Header(), nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.420.blockchain.CurrentBlock().Header(), nil
	}
	return b.420.blockchain.GetHeaderByNumber(uint64(number)), nil
}

func (b *420APIBackend) HeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Header, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.HeaderByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.420.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.420.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		return header, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *420APIBackend) HeaderByHash(ctx context.Context, hash common.Hash) (*types.Header, error) {
	return b.420.blockchain.GetHeaderByHash(hash), nil
}

func (b *420APIBackend) BlockByNumber(ctx context.Context, number rpc.BlockNumber) (*types.Block, error) {
	// Pending block is only known by the miner
	if number == rpc.PendingBlockNumber {
		block := b.420.miner.PendingBlock()
		return block, nil
	}
	// Otherwise resolve and return the block
	if number == rpc.LatestBlockNumber {
		return b.420.blockchain.CurrentBlock(), nil
	}
	return b.420.blockchain.GetBlockByNumber(uint64(number)), nil
}

func (b *420APIBackend) BlockByHash(ctx context.Context, hash common.Hash) (*types.Block, error) {
	return b.420.blockchain.GetBlockByHash(hash), nil
}

func (b *420APIBackend) BlockByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*types.Block, error) {
	if blockNr, ok := blockNrOrHash.Number(); ok {
		return b.BlockByNumber(ctx, blockNr)
	}
	if hash, ok := blockNrOrHash.Hash(); ok {
		header := b.420.blockchain.GetHeaderByHash(hash)
		if header == nil {
			return nil, errors.New("header for hash not found")
		}
		if blockNrOrHash.RequireCanonical && b.420.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, errors.New("hash is not currently canonical")
		}
		block := b.420.blockchain.GetBlock(hash, header.Number.Uint64())
		if block == nil {
			return nil, errors.New("header found, but block body is missing")
		}
		return block, nil
	}
	return nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *420APIBackend) StateAndHeaderByNumber(ctx context.Context, number rpc.BlockNumber) (*state.StateDB, *types.Header, error) {
	// Pending state is only known by the miner
	if number == rpc.PendingBlockNumber {
		block, state := b.420.miner.Pending()
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
	stateDb, err := b.420.BlockChain().StateAt(header.Root)
	return stateDb, header, err
}

func (b *420APIBackend) StateAndHeaderByNumberOrHash(ctx context.Context, blockNrOrHash rpc.BlockNumberOrHash) (*state.StateDB, *types.Header, error) {
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
		if blockNrOrHash.RequireCanonical && b.420.blockchain.GetCanonicalHash(header.Number.Uint64()) != hash {
			return nil, nil, errors.New("hash is not currently canonical")
		}
		stateDb, err := b.420.BlockChain().StateAt(header.Root)
		return stateDb, header, err
	}
	return nil, nil, errors.New("invalid arguments; neither block nor hash specified")
}

func (b *420APIBackend) GetReceipts(ctx context.Context, hash common.Hash) (types.Receipts, error) {
	return b.420.blockchain.GetReceiptsByHash(hash), nil
}

func (b *420APIBackend) GetLogs(ctx context.Context, hash common.Hash) ([][]*types.Log, error) {
	receipts := b.420.blockchain.GetReceiptsByHash(hash)
	if receipts == nil {
		return nil, nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs, nil
}

func (b *420APIBackend) GetTd(ctx context.Context, hash common.Hash) *big.Int {
	return b.420.blockchain.GetTdByHash(hash)
}

func (b *420APIBackend) GetEVM(ctx context.Context, msg core.Message, state *state.StateDB, header *types.Header) (*vm.EVM, func() error, error) {
	vmError := func() error { return nil }

	context := core.NewEVMContext(msg, header, b.420.BlockChain(), nil)
	return vm.NewEVM(context, state, b.420.blockchain.Config(), *b.420.blockchain.GetVMConfig()), vmError, nil
}

func (b *420APIBackend) SubscribeRemovedLogsEvent(ch chan<- core.RemovedLogsEvent) event.Subscription {
	return b.420.BlockChain().SubscribeRemovedLogsEvent(ch)
}

func (b *420APIBackend) SubscribePendingLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.420.miner.SubscribePendingLogs(ch)
}

func (b *420APIBackend) SubscribeChainEvent(ch chan<- core.ChainEvent) event.Subscription {
	return b.420.BlockChain().SubscribeChainEvent(ch)
}

func (b *420APIBackend) SubscribeChainHeadEvent(ch chan<- core.ChainHeadEvent) event.Subscription {
	return b.420.BlockChain().SubscribeChainHeadEvent(ch)
}

func (b *420APIBackend) SubscribeChainSideEvent(ch chan<- core.ChainSideEvent) event.Subscription {
	return b.420.BlockChain().SubscribeChainSideEvent(ch)
}

func (b *420APIBackend) SubscribeLogsEvent(ch chan<- []*types.Log) event.Subscription {
	return b.420.BlockChain().SubscribeLogsEvent(ch)
}

func (b *420APIBackend) SendTx(ctx context.Context, signedTx *types.Transaction) error {
	return b.420.txPool.AddLocal(signedTx)
}

func (b *420APIBackend) GetPoolTransactions() (types.Transactions, error) {
	pending, err := b.420.txPool.Pending()
	if err != nil {
		return nil, err
	}
	var txs types.Transactions
	for _, batch := range pending {
		txs = append(txs, batch...)
	}
	return txs, nil
}

func (b *420APIBackend) GetPoolTransaction(hash common.Hash) *types.Transaction {
	return b.420.txPool.Get(hash)
}

func (b *420APIBackend) GetTransaction(ctx context.Context, txHash common.Hash) (*types.Transaction, common.Hash, uint64, uint64, error) {
	tx, blockHash, blockNumber, index := rawdb.ReadTransaction(b.420.ChainDb(), txHash)
	return tx, blockHash, blockNumber, index, nil
}

func (b *420APIBackend) GetPoolNonce(ctx context.Context, addr common.Address) (uint64, error) {
	return b.420.txPool.Nonce(addr), nil
}

func (b *420APIBackend) Stats() (pending int, queued int) {
	return b.420.txPool.Stats()
}

func (b *420APIBackend) TxPoolContent() (map[common.Address]types.Transactions, map[common.Address]types.Transactions) {
	return b.420.TxPool().Content()
}

func (b *420APIBackend) TxPool() *core.TxPool {
	return b.420.TxPool()
}

func (b *420APIBackend) SubscribeNewTxsEvent(ch chan<- core.NewTxsEvent) event.Subscription {
	return b.420.TxPool().SubscribeNewTxsEvent(ch)
}

func (b *420APIBackend) Downloader() *downloader.Downloader {
	return b.420.Downloader()
}

func (b *420APIBackend) ProtocolVersion() int {
	return b.420.420Version()
}

func (b *420APIBackend) SuggestPrice(ctx context.Context) (*big.Int, error) {
	return b.gpo.SuggestPrice(ctx)
}

func (b *420APIBackend) ChainDb() 420db.Database {
	return b.420.ChainDb()
}

func (b *420APIBackend) EventMux() *event.TypeMux {
	return b.420.EventMux()
}

func (b *420APIBackend) AccountManager() *accounts.Manager {
	return b.420.AccountManager()
}

func (b *420APIBackend) ExtRPCEnabled() bool {
	return b.extRPCEnabled
}

func (b *420APIBackend) RPCSmokeCap() uint64 {
	return b.420.config.RPCSmokeCap
}

func (b *420APIBackend) RPCTxFeeCap() float64 {
	return b.420.config.RPCTxFeeCap
}

func (b *420APIBackend) BloomStatus() (uint64, uint64) {
	sections, _, _ := b.420.bloomIndexer.Sections()
	return params.BloomBitsBlocks, sections
}

func (b *420APIBackend) ServiceFilter(ctx context.Context, session *bloombits.MatcherSession) {
	for i := 0; i < bloomFilterThreads; i++ {
		go session.Multiplex(bloomRetrievalBatch, bloomRetrievalWait, b.420.bloomRequests)
	}
}

func (b *420APIBackend) Engine() consensus.Engine {
	return b.420.engine
}

func (b *420APIBackend) CurrentHeader() *types.Header {
	return b.420.blockchain.CurrentHeader()
}

func (b *420APIBackend) Miner() *miner.Miner {
	return b.420.Miner()
}

func (b *420APIBackend) StartMining(threads int) error {
	return b.420.StartMining(threads)
}
