// Copyright 2018 The The 420Integrated Development Group
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

package les

import (
	"fmt"
	"math/big"
	"sync"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/core"
	"github.com/420integrated/go-420coin/core/rawdb"
	"github.com/420integrated/go-420coin/core/types"
	"github.com/420integrated/go-420coin/420"
	"github.com/420integrated/go-420coin/420client"
	"github.com/420integrated/go-420coin/420db"
	"github.com/420integrated/go-420coin/les/checkpointoracle"
	"github.com/420integrated/go-420coin/light"
	"github.com/420integrated/go-420coin/log"
	"github.com/420integrated/go-420coin/node"
	"github.com/420integrated/go-420coin/p2p"
	"github.com/420integrated/go-420coin/p2p/discv5"
	"github.com/420integrated/go-420coin/p2p/enode"
	"github.com/420integrated/go-420coin/params"
)

func errResp(code errCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

func lesTopic(genesisHash common.Hash, protocolVersion uint) discv5.Topic {
	var name string
	switch protocolVersion {
	case lpv2:
		name = "LES2"
	default:
		panic(nil)
	}
	return discv5.Topic(name + "@" + common.Bytes2Hex(genesisHash.Bytes()[0:8]))
}

type chainReader interface {
	CurrentHeader() *types.Header
}

// lesCommons contains fields needed by both server and client.
type lesCommons struct {
	genesis                      common.Hash
	config                       *420.Config
	chainConfig                  *params.ChainConfig
	iConfig                      *light.IndexerConfig
	chainDb                      420db.Database
	chainReader                  chainReader
	chtIndexer, bloomTrieIndexer *core.ChainIndexer
	oracle                       *checkpointoracle.CheckpointOracle

	closeCh chan struct{}
	wg      sync.WaitGroup
}

// NodeInfo represents a short summary of the 420coin sub-protocol metadata
// known about the host peer.
type NodeInfo struct {
	Network    uint64                   `json:"network"`    // 420coin network ID (1=Frontier, 2=Morden, Ropsten=3, Rinkeby=4)
	Difficulty *big.Int                 `json:"difficulty"` // Total difficulty of the host's blockchain
	Genesis    common.Hash              `json:"genesis"`    // SHA3 hash of the host's genesis block
	Config     *params.ChainConfig      `json:"config"`     // Chain configuration for the fork rules
	Head       common.Hash              `json:"head"`       // SHA3 hash of the host's best owned block
	CHT        params.TrustedCheckpoint `json:"cht"`        // Trused CHT checkpoint for fast catchup
}

// makeProtocols creates protocol descriptors for the given LES versions.
func (c *lesCommons) makeProtocols(versions []uint, runPeer func(version uint, p *p2p.Peer, rw p2p.MsgReadWriter) error, peerInfo func(id enode.ID) interface{}, dialCandidates enode.Iterator) []p2p.Protocol {
	protos := make([]p2p.Protocol, len(versions))
	for i, version := range versions {
		version := version
		protos[i] = p2p.Protocol{
			Name:     "les",
			Version:  version,
			Length:   ProtocolLengths[version],
			NodeInfo: c.nodeInfo,
			Run: func(peer *p2p.Peer, rw p2p.MsgReadWriter) error {
				return runPeer(version, peer, rw)
			},
			PeerInfo:       peerInfo,
			DialCandidates: dialCandidates,
		}
	}
	return protos
}

// nodeInfo retrieves some protocol metadata about the running host node.
func (c *lesCommons) nodeInfo() interface{} {
	head := c.chainReader.CurrentHeader()
	hash := head.Hash()
	return &NodeInfo{
		Network:    c.config.NetworkId,
		Difficulty: rawdb.ReadTd(c.chainDb, hash, head.Number.Uint64()),
		Genesis:    c.genesis,
		Config:     c.chainConfig,
		Head:       hash,
		CHT:        c.latestLocalCheckpoint(),
	}
}

// latestLocalCheckpoint finds the common stored section index and returns a set
// of post-processed trie roots (CHT and BloomTrie) associated with the appropriate
// section index and head hash as a local checkpoint package.
func (c *lesCommons) latestLocalCheckpoint() params.TrustedCheckpoint {
	sections, _, _ := c.chtIndexer.Sections()
	sections2, _, _ := c.bloomTrieIndexer.Sections()
	// Cap the section index if the two sections are not consistent.
	if sections > sections2 {
		sections = sections2
	}
	if sections == 0 {
		// No checkpoint information can be provided.
		return params.TrustedCheckpoint{}
	}
	return c.localCheckpoint(sections - 1)
}

// localCheckpoint returns a set of post-processed trie roots (CHT and BloomTrie)
// associated with the appropriate head hash by specific section index.
//
// The returned checkpoint is only the checkpoint generated by the local indexers,
// not the stable checkpoint registered in the registrar contract.
func (c *lesCommons) localCheckpoint(index uint64) params.TrustedCheckpoint {
	sectionHead := c.chtIndexer.SectionHead(index)
	return params.TrustedCheckpoint{
		SectionIndex: index,
		SectionHead:  sectionHead,
		CHTRoot:      light.GetChtRoot(c.chainDb, index, sectionHead),
		BloomRoot:    light.GetBloomTrieRoot(c.chainDb, index, sectionHead),
	}
}

// setupOracle sets up the checkpoint oracle contract client.
func (c *lesCommons) setupOracle(node *node.Node, genesis common.Hash, 420config *420.Config) *checkpointoracle.CheckpointOracle {
	config := 420config.CheckpointOracle
	if config == nil {
		// Try loading default config.
		config = params.CheckpointOracles[genesis]
	}
	if config == nil {
		log.Info("Checkpoint registrar is not enabled")
		return nil
	}
	if config.Address == (common.Address{}) || uint64(len(config.Signers)) < config.Threshold {
		log.Warn("Invalid checkpoint registrar config")
		return nil
	}
	oracle := checkpointoracle.New(config, c.localCheckpoint)
	rpcClient, _ := node.Attach()
	client := 420client.NewClient(rpcClient)
	oracle.Start(client)
	log.Info("Configured checkpoint registrar", "address", config.Address, "signers", len(config.Signers), "threshold", config.Threshold)
	return oracle
}
