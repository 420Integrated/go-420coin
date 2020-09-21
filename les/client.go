// Copyright 2016 The The 420Integrated Development Group
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

// Package les implements the Light 420coin Subprotocol.
package les

import (
	"fmt"
	"time"

	"github.com/420integrated/go-420coin/accounts"
	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/common/hexutil"
	"github.com/420integrated/go-420coin/common/mclock"
	"github.com/420integrated/go-420coin/consensus"
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/core/bloombits"
	"github.com/420integrated/go-420coin/core/rawdb"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/420"
	"github.com/420integrated/go-420coin/420/downloader"
	"github.com/420integrated/go-420coin/420/filters"
	"github.com/420integrated/go-420coin/420/smokeprice"
	"github.com/420integrated/go-420coin/event"
	"github.com/420integrated/go-420coin/internal/420api"
	lpc "github.com/420integrated/go-420coin/les/lespay/client"
	"github.com/420integrated/go-420coin/light"
	"github.com/420integrated/go-420coin/log"
	"github.com/420integrated/go-420coin/node"
	"github.com/420integrated/go-420coin/p2p"
	"github.com/420integrated/go-420coin/p2p/enode"
	"github.com/420integrated/go-420coin/params"
	"github.com/420integrated/go-420coin/rpc"
)

type Light420coin struct {
	lesCommons

	peers          *serverPeerSet
	reqDist        *requestDistributor
	retriever      *retrieveManager
	odr            *LesOdr
	relay          *lesTxRelay
	handler        *clientHandler
	txPool         *light.TxPool
	blockchain     *light.LightChain
	serverPool     *serverPool
	valueTracker   *lpc.ValueTracker
	dialCandidates enode.Iterator
	pruner         *pruner

	bloomRequests chan chan *bloombits.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *core.ChainIndexer             // Bloom indexer operating during block imports

	ApiBackend     *LesApiBackend
	eventMux       *event.TypeMux
	engine         consensus.Engine
	accountManager *accounts.Manager
	netRPCService  *420api.PublicNetAPI

	p2pServer *p2p.Server
}

// New creates an instance of the light client.
func New(stack *node.Node, config *420.Config) (*Light420coin, error) {
	chainDb, err := stack.OpenDatabase("lightchaindata", config.DatabaseCache, config.DatabaseHandles, "420/db/chaindata/")
	if err != nil {
		return nil, err
	}
	lespayDb, err := stack.OpenDatabase("lespay", 0, 0, "420/db/lespay")
	if err != nil {
		return nil, err
	}
	chainConfig, genesisHash, genesisErr := core.SetupGenesisBlock(chainDb, config.Genesis)
	if _, isCompat := genesisErr.(*params.ConfigCompatError); genesisErr != nil && !isCompat {
		return nil, genesisErr
	}
	log.Info("Initialised chain configuration", "config", chainConfig)

	peers := newServerPeerSet()
	l420 := &Light420coin{
		lesCommons: lesCommons{
			genesis:     genesisHash,
			config:      config,
			chainConfig: chainConfig,
			iConfig:     light.DefaultClientIndexerConfig,
			chainDb:     chainDb,
			closeCh:     make(chan struct{}),
		},
		peers:          peers,
		eventMux:       stack.EventMux(),
		reqDist:        newRequestDistributor(peers, &mclock.System{}),
		accountManager: stack.AccountManager(),
		engine:         420.CreateConsensusEngine(stack, chainConfig, &config.Ethash, nil, false, chainDb),
		bloomRequests:  make(chan chan *bloombits.Retrieval),
		bloomIndexer:   420.NewBloomIndexer(chainDb, params.BloomBitsBlocksClient, params.HelperTrieConfirmations),
		valueTracker:   lpc.NewValueTracker(lespayDb, &mclock.System{}, requestList, time.Minute, 1/float64(time.Hour), 1/float64(time.Hour*100), 1/float64(time.Hour*1000)),
		p2pServer:      stack.Server(),
	}
	peers.subscribe((*vtSubscription)(l420.valueTracker))

	dnsdisc, err := l420.setupDiscovery(&stack.Config().P2P)
	if err != nil {
		return nil, err
	}
	l420.serverPool = newServerPool(lespayDb, []byte("serverpool:"), l420.valueTracker, dnsdisc, time.Second, nil, &mclock.System{}, config.UltraLightServers)
	peers.subscribe(l420.serverPool)
	l420.dialCandidates = l420.serverPool.dialIterator

	l420.retriever = newRetrieveManager(peers, l420.reqDist, l420.serverPool.getTimeout)
	l420.relay = newLesTxRelay(peers, l420.retriever)

	l420.odr = NewLesOdr(chainDb, light.DefaultClientIndexerConfig, l420.retriever)
	l420.chtIndexer = light.NewChtIndexer(chainDb, l420.odr, params.CHTFrequency, params.HelperTrieConfirmations, config.LightNoPrune)
	l420.bloomTrieIndexer = light.NewBloomTrieIndexer(chainDb, l420.odr, params.BloomBitsBlocksClient, params.BloomTrieFrequency, config.LightNoPrune)
	l420.odr.SetIndexers(l420.chtIndexer, l420.bloomTrieIndexer, l420.bloomIndexer)

	checkpoint := config.Checkpoint
	if checkpoint == nil {
		checkpoint = params.TrustedCheckpoints[genesisHash]
	}
	// Note: NewLightChain adds the trusted checkpoint so it needs an ODR with
	// indexers already set but not started yet
	if l420.blockchain, err = light.NewLightChain(l420.odr, l420.chainConfig, l420.engine, checkpoint); err != nil {
		return nil, err
	}
	l420.chainReader = l420.blockchain
	l420.txPool = light.NewTxPool(l420.chainConfig, l420.blockchain, l420.relay)

	// Set up checkpoint oracle.
	l420.oracle = l420.setupOracle(stack, genesisHash, config)

	// Note: AddChildIndexer starts the update process for the child
	l420.bloomIndexer.AddChildIndexer(l420.bloomTrieIndexer)
	l420.chtIndexer.Start(l420.blockchain)
	l420.bloomIndexer.Start(l420.blockchain)

	// Start a light chain pruner to delete useless historical data.
	l420.pruner = newPruner(chainDb, l420.chtIndexer, l420.bloomTrieIndexer)

	// Rewind the chain in case of an incompatible config upgrade.
	if compat, ok := genesisErr.(*params.ConfigCompatError); ok {
		log.Warn("Rewinding chain to upgrade configuration", "err", compat)
		l420.blockchain.SetHead(compat.RewindTo)
		rawdb.WriteChainConfig(chainDb, genesisHash, chainConfig)
	}

	l420.ApiBackend = &LesApiBackend{stack.Config().ExtRPCEnabled(), l420, nil}
	gpoParams := config.GPO
	if gpoParams.Default == nil {
		gpoParams.Default = config.Miner.SmokePrice
	}
	l420.ApiBackend.gpo = smokeprice.NewOracle(l420.ApiBackend, gpoParams)

	l420.handler = newClientHandler(config.UltraLightServers, config.UltraLightFraction, checkpoint, l420)
	if l420.handler.ulc != nil {
		log.Warn("Ultra light client is enabled", "trustedNodes", len(l420.handler.ulc.keys), "minTrustedFraction", l420.handler.ulc.fraction)
		l420.blockchain.DisableCheckFreq()
	}

	l420.netRPCService = 420api.NewPublicNetAPI(l420.p2pServer, l420.config.NetworkId)

	// Register the backend on the node
	stack.RegisterAPIs(l420.APIs())
	stack.RegisterProtocols(l420.Protocols())
	stack.RegisterLifecycle(l420)

	return l420, nil
}

// vtSubscription implements serverPeerSubscriber
type vtSubscription lpc.ValueTracker

// registerPeer implements serverPeerSubscriber
func (v *vtSubscription) registerPeer(p *serverPeer) {
	vt := (*lpc.ValueTracker)(v)
	p.setValueTracker(vt, vt.Register(p.ID()))
	p.updateVtParams()
}

// unregisterPeer implements serverPeerSubscriber
func (v *vtSubscription) unregisterPeer(p *serverPeer) {
	vt := (*lpc.ValueTracker)(v)
	vt.Unregister(p.ID())
	p.setValueTracker(nil, nil)
}

type LightDummyAPI struct{}

// 420coinbase is the address that mining rewards will be send to
func (s *LightDummyAPI) 420coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Coinbase is the address that mining rewards will be send to (alias for 420coinbase)
func (s *LightDummyAPI) Coinbase() (common.Address, error) {
	return common.Address{}, fmt.Errorf("mining is not supported in light mode")
}

// Hashrate returns the POW hashrate
func (s *LightDummyAPI) Hashrate() hexutil.Uint {
	return 0
}

// Mining returns an indication if this node is currently mining.
func (s *LightDummyAPI) Mining() bool {
	return false
}

// APIs returns the collection of RPC services the 420coin package offers.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Light420coin) APIs() []rpc.API {
	apis := 420api.GetAPIs(s.ApiBackend)
	apis = append(apis, s.engine.APIs(s.BlockChain().HeaderChain())...)
	return append(apis, []rpc.API{
		{
			Namespace: "420",
			Version:   "1.0",
			Service:   &LightDummyAPI{},
			Public:    true,
		}, {
			Namespace: "420",
			Version:   "1.0",
			Service:   downloader.NewPublicDownloaderAPI(s.handler.downloader, s.eventMux),
			Public:    true,
		}, {
			Namespace: "420",
			Version:   "1.0",
			Service:   filters.NewPublicFilterAPI(s.ApiBackend, true),
			Public:    true,
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   s.netRPCService,
			Public:    true,
		}, {
			Namespace: "les",
			Version:   "1.0",
			Service:   NewPrivateLightAPI(&s.lesCommons),
			Public:    false,
		}, {
			Namespace: "lespay",
			Version:   "1.0",
			Service:   lpc.NewPrivateClientAPI(s.valueTracker),
			Public:    false,
		},
	}...)
}

func (s *Light420coin) ResetWithGenesisBlock(gb *types.Block) {
	s.blockchain.ResetWithGenesisBlock(gb)
}

func (s *Light420coin) BlockChain() *light.LightChain      { return s.blockchain }
func (s *Light420coin) TxPool() *light.TxPool              { return s.txPool }
func (s *Light420coin) Engine() consensus.Engine           { return s.engine }
func (s *Light420coin) LesVersion() int                    { return int(ClientProtocolVersions[0]) }
func (s *Light420coin) Downloader() *downloader.Downloader { return s.handler.downloader }
func (s *Light420coin) EventMux() *event.TypeMux           { return s.eventMux }

// Protocols returns all the currently configured network protocols to start.
func (s *Light420coin) Protocols() []p2p.Protocol {
	return s.makeProtocols(ClientProtocolVersions, s.handler.runPeer, func(id enode.ID) interface{} {
		if p := s.peers.peer(id.String()); p != nil {
			return p.Info()
		}
		return nil
	}, s.dialCandidates)
}

// Start implements node.Lifecycle, starting all internal goroutines needed by the
// light 420coin protocol implementation.
func (s *Light420coin) Start() error {
	log.Warn("Light client mode is an experimental feature")

	s.serverPool.start()
	// Start bloom request workers.
	s.wg.Add(bloomServiceThreads)
	s.startBloomHandlers(params.BloomBitsBlocksClient)
	s.handler.start()

	return nil
}

// Stop implements node.Lifecycle, terminating all internal goroutines used by the
// 420coin protocol.
func (s *Light420coin) Stop() error {
	close(s.closeCh)
	s.serverPool.stop()
	s.valueTracker.Stop()
	s.peers.close()
	s.reqDist.close()
	s.odr.Stop()
	s.relay.Stop()
	s.bloomIndexer.Close()
	s.chtIndexer.Close()
	s.blockchain.Stop()
	s.handler.stop()
	s.txPool.Stop()
	s.engine.Close()
	s.pruner.close()
	s.eventMux.Stop()
	s.chainDb.Close()
	s.wg.Wait()
	log.Info("Light 420coin stopped")
	return nil
}
