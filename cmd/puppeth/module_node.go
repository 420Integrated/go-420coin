// Copyright 2017 The The 420Integrated Development Group
// This file is part of go-420coin.
//
// go-420coin is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-420coin is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-420coin. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/420integrated/go-420coin/common"
	"github.com/420integrated/go-420coin/log"
)

// nodeDockerfile is the Dockerfile required to run an 420coin node.
var nodeDockerfile = `
FROM fourtwentycoin/client-go:latest

ADD genesis.json /genesis.json
{{if .Unlock}}
	ADD signer.json /signer.json
	ADD signer.pass /signer.pass
{{end}}
RUN \
  echo 'g420 --cache 512 init /genesis.json' > g420.sh && \{{if .Unlock}}
	echo 'mkdir -p /root/.420coin/keystore/ && cp /signer.json /root/.420coin/keystore/' >> g420.sh && \{{end}}
	echo $'exec g420 --networkid {{.NetworkID}} --cache 512 --port {{.Port}} --nat extip:{{.IP}} --maxpeers {{.Peers}} {{.LightFlag}} --fourtwentystats \'{{.fourtwentystats}}\' {{if .Bootnodes}}--bootnodes {{.Bootnodes}}{{end}} {{if .Fourtwentycoinbase}}--miner.Fourtwentycoinbase {{.Fourtwentycoinbase}} --mine --miner.threads 1{{end}} {{if .Unlock}}--unlock 0 --password /signer.pass --mine{{end}} --miner.smoketarget {{.SmokeTarget}} --miner.smokelimit {{.SmokeLimit}} --miner.smokeprice {{.SmokePrice}}' >> g420.sh

ENTRYPOINT ["/bin/sh", "g420.sh"]
`

// nodeComposefile is the docker-compose.yml file required to deploy and maintain
// an 420coin node (bootnode or miner for now).
var nodeComposefile = `
version: '2'
services:
  {{.Type}}:
    build: .
    image: {{.Network}}/{{.Type}}
    container_name: {{.Network}}_{{.Type}}_1
    ports:
      - "{{.Port}}:{{.Port}}"
      - "{{.Port}}:{{.Port}}/udp"
    volumes:
      - {{.Datadir}}:/root/.420coin{{if .Ethashdir}}
      - {{.Ethashdir}}:/root/.ethash{{end}}
    environment:
      - PORT={{.Port}}/tcp
      - TOTAL_PEERS={{.TotalPeers}}
      - LIGHT_PEERS={{.LightPeers}}
      - STATS_NAME={{.fourtwentystats}}
      - MINER_NAME={{.Fourtwentycoinbase}}
      - SMOKE_TARGET={{.SmokeTarget}}
      - SMOKE_LIMIT={{.SmokeLimit}}
      - SMOKE_PRICE={{.SmokePrice}}
    logging:
      driver: "json-file"
      options:
        max-size: "1m"
        max-file: "10"
    restart: always
`

// deployNode deploys a new 420coin node container to a remote machine via SSH,
// docker and docker-compose. If an instance with the specified network name
// already exists there, it will be overwritten!
func deployNode(client *sshClient, network string, bootnodes []string, config *nodeInfos, nocache bool) ([]byte, error) {
	kind := "sealnode"
	if config.keyJSON == "" && config.Fourtwentycoinbase == "" {
		kind = "bootnode"
		bootnodes = make([]string, 0)
	}
	// Generate the content to upload to the server
	workdir := fmt.Sprintf("%d", rand.Int63())
	files := make(map[string][]byte)

	lightFlag := ""
	if config.peersLight > 0 {
		lightFlag = fmt.Sprintf("--lightpeers=%d --lightserv=50", config.peersLight)
	}
	dockerfile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nodeDockerfile)).Execute(dockerfile, map[string]interface{}{
		"NetworkID":          config.network,
		"Port":               config.port,
		"IP":                 client.address,
		"Peers":              config.peersTotal,
		"LightFlag":          lightFlag,
		"Bootnodes":          strings.Join(bootnodes, ","),
		"fourtwentystats":    config.fourtwentystats,
		"Fourtwentycoinbase": config.Fourtwentycoinbase,
		"SmokeTarget":        uint64(1000000 * config.smokeTarget),
		"SmokeLimit":         uint64(1000000 * config.smokeLimit),
		"SmokePrice":         uint64(1000000000 * config.smokePrice),
		"Unlock":             config.keyJSON != "",
	})
	files[filepath.Join(workdir, "Dockerfile")] = dockerfile.Bytes()

	composefile := new(bytes.Buffer)
	template.Must(template.New("").Parse(nodeComposefile)).Execute(composefile, map[string]interface{}{
		"Type":                kind,
		"Datadir":             config.datadir,
		"Ethashdir":           config.ethashdir,
		"Network":             network,
		"Port":                config.port,
		"TotalPeers":          config.peersTotal,
		"Light":               config.peersLight > 0,
		"LightPeers":          config.peersLight,
		"fourtwentystats":     config.fourtwentystats[:strings.Index(config.fourtwentystats, ":")],
		"Fourtwentycoinbase":  config.Fourtwentycoinbase,
		"SmokeTarget":         config.smokeTarget,
		"SmokeLimit":          config.smokeLimit,
		"SmokePrice":          config.smokePrice,
	})
	files[filepath.Join(workdir, "docker-compose.yaml")] = composefile.Bytes()

	files[filepath.Join(workdir, "genesis.json")] = config.genesis
	if config.keyJSON != "" {
		files[filepath.Join(workdir, "signer.json")] = []byte(config.keyJSON)
		files[filepath.Join(workdir, "signer.pass")] = []byte(config.keyPass)
	}
	// Upload the deployment files to the remote server (and clean up afterwards)
	if out, err := client.Upload(files); err != nil {
		return out, err
	}
	defer client.Run("rm -rf " + workdir)

	// Build and deploy the boot or seal node service
	if nocache {
		return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s build --pull --no-cache && docker-compose -p %s up -d --force-recreate --timeout 60", workdir, network, network))
	}
	return nil, client.Stream(fmt.Sprintf("cd %s && docker-compose -p %s up -d --build --force-recreate --timeout 60", workdir, network))
}

// nodeInfos is returned from a boot or seal node status check to allow reporting
// various configuration parameters.
type nodeInfos struct {
	genesis             []byte
	network             int64
	datadir             string
	ethashdir           string
	fourtwentystats     string
	port                int
	enode               string
	peersTotal          int
	peersLight          int
	Fourtwentycoinbase  string
	keyJSON             string
	keyPass             string
	smokeTarget         float64
	smokeLimit          float64
	smokePrice          float64
}

// Report converts the typed struct into a plain string->string map, containing
// most - but not all - fields for reporting to the user.
func (info *nodeInfos) Report() map[string]string {
	report := map[string]string{
		"Data directory":           info.datadir,
		"Listener port":            strconv.Itoa(info.port),
		"Peer count (all total)":   strconv.Itoa(info.peersTotal),
		"Peer count (light nodes)": strconv.Itoa(info.peersLight),
		"420stats username":        info.fourtwentystats,
	}
	if info.smokeTarget > 0 {
		// Miner or signer node
		report["Smoke price (minimum accepted)"] = fmt.Sprintf("%0.3f Maher", info.smokePrice)
		report["Smoke floor (baseline target)"] = fmt.Sprintf("%0.3f MSmoke", info.smokeTarget)
		report["Smoke ceil  (target maximum)"] = fmt.Sprintf("%0.3f MSmoke", info.smokeLimit)

		if info.fourtwentycoinbase != "" {
			// Ethash proof-of-work miner
			report["Ethash directory"] = info.ethashdir
			report["Miner account"] = info.fourtwentycoinbase
		}
		if info.keyJSON != "" {
			// Clique proof-of-authority signer
			var key struct {
				Address string `json:"address"`
			}
			if err := json.Unmarshal([]byte(info.keyJSON), &key); err == nil {
				report["Signer account"] = common.HexToAddress(key.Address).Hex()
			} else {
				log.Error("Failed to retrieve signer address", "err", err)
			}
		}
	}
	return report
}

// checkNode does a health-check against a boot or seal node server to verify
// if it's running, and if yes, if it's responsive.
func checkNode(client *sshClient, network string, boot bool) (*nodeInfos, error) {
	kind := "bootnode"
	if !boot {
		kind = "sealnode"
	}
	// Inspect a possible bootnode container on the host
	infos, err := inspectContainer(client, fmt.Sprintf("%s_%s_1", network, kind))
	if err != nil {
		return nil, err
	}
	if !infos.running {
		return nil, ErrServiceOffline
	}
	// Resolve a few types from the environmental variables
	totalPeers, _ := strconv.Atoi(infos.envvars["TOTAL_PEERS"])
	lightPeers, _ := strconv.Atoi(infos.envvars["LIGHT_PEERS"])
	smokeTarget, _ := strconv.ParseFloat(infos.envvars["SMOKE_TARGET"], 64)
	smokeLimit, _ := strconv.ParseFloat(infos.envvars["SMOKE_LIMIT"], 64)
	smokePrice, _ := strconv.ParseFloat(infos.envvars["SMOKE_PRICE"], 64)

	// Container available, retrieve its node ID and its genesis json
	var out []byte
	if out, err = client.Run(fmt.Sprintf("docker exec %s_%s_1 g420 --exec admin.nodeInfo.enode --cache=16 attach", network, kind)); err != nil {
		return nil, ErrServiceUnreachable
	}
	enode := bytes.Trim(bytes.TrimSpace(out), "\"")

	if out, err = client.Run(fmt.Sprintf("docker exec %s_%s_1 cat /genesis.json", network, kind)); err != nil {
		return nil, ErrServiceUnreachable
	}
	genesis := bytes.TrimSpace(out)

	keyJSON, keyPass := "", ""
	if out, err = client.Run(fmt.Sprintf("docker exec %s_%s_1 cat /signer.json", network, kind)); err == nil {
		keyJSON = string(bytes.TrimSpace(out))
	}
	if out, err = client.Run(fmt.Sprintf("docker exec %s_%s_1 cat /signer.pass", network, kind)); err == nil {
		keyPass = string(bytes.TrimSpace(out))
	}
	// Run a sanity check to see if the devp2p is reachable
	port := infos.portmap[infos.envvars["PORT"]]
	if err = checkPort(client.server, port); err != nil {
		log.Warn(fmt.Sprintf("%s devp2p port seems unreachable", strings.Title(kind)), "server", client.server, "port", port, "err", err)
	}
	// Assemble and return the useful infos
	stats := &nodeInfos{
		genesis:    genesis,
		datadir:    infos.volumes["/root/.420coin"],
		ethashdir:  infos.volumes["/root/.ethash"],
		port:       port,
		peersTotal: totalPeers,
		peersLight: lightPeers,
		fourtwentystats:   infos.envvars["STATS_NAME"],
		fourtwentycoinbase:  infos.envvars["MINER_NAME"],
		keyJSON:    keyJSON,
		keyPass:    keyPass,
		smokeTarget:  smokeTarget,
		smokeLimit:   smokeLimit,
		smokePrice:   smokePrice,
	}
	stats.enode = string(enode)

	return stats, nil
}
