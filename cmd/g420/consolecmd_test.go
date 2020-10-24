// Copyright 2016 The The 420Integrated Development Group
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
	"crypto/rand"
	"math/big"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/420integrated/go-420coin/params"
)

const (
	ipcAPIs  = "admin:1.0 debug:1.0 420:1.0 ethash:1.0 miner:1.0 net:1.0 personal:1.0 rpc:1.0 txpool:1.0 web3:1.0"
	httpAPIs = "420:1.0 net:1.0 rpc:1.0 web3:1.0"
)

// Tests that a node embedded within a console can be started up properly and
// then terminated by closing the input stream.
func TestConsoleWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"

	// Start a g420 console, make sure it's cleaned up and terminate the console
	g420 := runG420(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--fourtwentycoinbase", coinbase,
		"console")

	// Gather all the infos the welcome message needs to contain
	g420.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	g420.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	g420.SetTemplateFunc("gover", runtime.Version)
	g420.SetTemplateFunc("g420ver", func() string { return params.VersionWithCommit("", "") })
	g420.SetTemplateFunc("niltime", func() string {
		return time.Unix(0, 0).Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	})
	g420.SetTemplateFunc("apis", func() string { return ipcAPIs })

	// Verify the actual welcome message to the required template
	g420.Expect(`
Welcome to the G420 JavaScript console!

instance: G420/v{{g420ver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{.fourtwentycoinbase}}
at block: 0 ({{niltime}})
 datadir: {{.Datadir}}
 modules: {{apis}}

> {{.InputLine "exit"}}
`)
	g420.ExpectExit()
}

// Tests that a console can be attached to a running node via various means.
func TestIPCAttachWelcome(t *testing.T) {
	// Configure the instance for IPC attachment
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	var ipc string
	if runtime.GOOS == "windows" {
		ipc = `\\.\pipe\g420` + strconv.Itoa(trulyRandInt(100000, 999999))
	} else {
		ws := tmpdir(t)
		defer os.RemoveAll(ws)
		ipc = filepath.Join(ws, "g420.ipc")
	}
	g420 := runG420(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--fourtwentycoinbase", coinbase, "--ipcpath", ipc)

	defer func() {
		g420.Interrupt()
		g420.ExpectExit()
	}()

	waitForEndpoint(t, ipc, 3*time.Second)
	testAttachWelcome(t, g420, "ipc:"+ipc, ipcAPIs)

}

func TestHTTPAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P
	g420 := runG420(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--fourtwentycoinbase", coinbase, "--http", "--http.port", port)
	defer func() {
		g420.Interrupt()
		g420.ExpectExit()
	}()

	endpoint := "http://127.0.0.1:" + port
	waitForEndpoint(t, endpoint, 3*time.Second)
	testAttachWelcome(t, g420, endpoint, httpAPIs)
}

func TestWSAttachWelcome(t *testing.T) {
	coinbase := "0x8605cdbbdb6d264aa742e77020dcbc58fcdce182"
	port := strconv.Itoa(trulyRandInt(1024, 65536)) // Yeah, sometimes this will fail, sorry :P

	g420 := runG420(t,
		"--port", "0", "--maxpeers", "0", "--nodiscover", "--nat", "none",
		"--fourtwentycoinbase", coinbase, "--ws", "--ws.port", port)
	defer func() {
		g420.Interrupt()
		g420.ExpectExit()
	}()

	endpoint := "ws://127.0.0.1:" + port
	waitForEndpoint(t, endpoint, 3*time.Second)
	testAttachWelcome(t, g420, endpoint, httpAPIs)
}

func testAttachWelcome(t *testing.T, g420 *testg420, endpoint, apis string) {
	// Attach to a running g420 note and terminate immediately
	attach := runG420(t, "attach", endpoint)
	defer attach.ExpectExit()
	attach.CloseStdin()

	// Gather all the infos the welcome message needs to contain
	attach.SetTemplateFunc("goos", func() string { return runtime.GOOS })
	attach.SetTemplateFunc("goarch", func() string { return runtime.GOARCH })
	attach.SetTemplateFunc("gover", runtime.Version)
	attach.SetTemplateFunc("g420ver", func() string { return params.VersionWithCommit("", "") })
	attach.SetTemplateFunc("fourtwentycoinbase", func() string { return g420.fourtwentycoinbase })
	attach.SetTemplateFunc("niltime", func() string {
		return time.Unix(0, 0).Format("Mon Jan 02 2006 15:04:05 GMT-0700 (MST)")
	})
	attach.SetTemplateFunc("ipc", func() bool { return strings.HasPrefix(endpoint, "ipc") })
	attach.SetTemplateFunc("datadir", func() string { return g420.Datadir })
	attach.SetTemplateFunc("apis", func() string { return apis })

	// Verify the actual welcome message to the required template
	attach.Expect(`
Welcome to the G420 JavaScript console!

instance: G420/v{{g420ver}}/{{goos}}-{{goarch}}/{{gover}}
coinbase: {{fourtwentycoinbase}}
at block: 0 ({{niltime}}){{if ipc}}
 datadir: {{datadir}}{{end}}
 modules: {{apis}}

> {{.InputLine "exit" }}
`)
	attach.ExpectExit()
}

// trulyRandInt generates a crypto random integer used by the console tests to
// not clash network ports with other tests running cocurrently.
func trulyRandInt(lo, hi int) int {
	num, _ := rand.Int(rand.Reader, big.NewInt(int64(hi-lo)))
	return int(num.Int64()) + lo
}
