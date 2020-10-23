## Go 420coin

Official Golang implementation of the 420coin protocol.

[![API Reference](
https://camo.githubusercontent.com/915b7be44ada53c290eb157634330494ebe3e30a/68747470733a2f2f676f646f632e6f72672f6769746875622e636f6d2f676f6c616e672f6764646f3f7374617475732e737667
)](https://pkg.go.dev/github.com/420integrated/go-420coin?tab=doc)
[![Go Report Card](https://goreportcard.com/badge/github.com/420integrated/go-420coin)](https://goreportcard.com/report/github.com/420integrated/go-420coin)
[![Travis](https://travis-ci.org/420integrated/go-420coin.svg?branch=master)](https://travis-ci.org/420integrated/go-420coin)
[![Discord](https://img.shields.io/badge/discord-join%20chat-blue.svg)](https://discord.gg/nthXNEv)

Automated builds are available for stable releases and the unstable master branch. Binary
archives are published at https://420integrated.com/downloads/.

## Building the source on Ubuntu 16.04

sudo apt-get update
sudo apt-get -y upgrade
sudo curl -O https://storage.googleapis.com/golang/go1.8.3.linux-amd64.tar.gz
sudo tar -xvf go1.8.3.linux-amd64.tar.gz
sudo mv go /usr/local
nano ~/.profile

Add export PATH=$PATH:/usr/local/go/bin to the file:

source ~/.profile
git clone https://github.com/420integrated/go-420coin.git
sudo apt-get install -y build-essential
cd go-420coin
make g420
./build/bin/g420 account new
./build/bin/g420

For prerequisites and detailed build instructions please read the [Installation Instructions](https://github.com/420integrated/go-420coin/wiki/Building-420coin) on the wiki.

Building `g420` requires both a Go (version 1.13 or later) and a C compiler. You can install
them using your favourite package manager. Once the dependencies are installed, run

```shell
make g420
```

or, to build the full suite of utilities:

```shell
make all
```

## Executables

The go-420coin project comes with several wrappers/executables found in the `cmd`
directory.

|    Command    | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| :-----------: | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
|  **`g420`**   | Our main 420coin CLI client. It is the entry point into the 420coin network (main-, test- or private net), capable of running as a full node (default), archive node (retaining all historical state) or a light node (retrieving data live). It can be used by other processes as a gateway into the 420coin network via JSON RPC endpoints exposed on top of HTTP, WebSocket and/or IPC transports. `g420 --help` and the [CLI Wiki page](https://github.com/420integrated/go-420coin/wiki/Command-Line-Options) for command line options.          |
|   `abigen`    | Source code generator to convert 420coin contract definitions into easy to use, compile-time type-safe Go packages. It operates on plain [420coin contract ABIs](https://github.com/420coin/wiki/wiki/420coin-Contract-ABI) with expanded functionality if the contract bytecode is also available. However, it also accepts Solidity source files, making development much more streamlined. Please see our [Native DApps](https://github.com/420integrated/go-420coin/wiki/Native-DApps:-Go-bindings-to-420coin-contracts) wiki page for details. |
|  `bootnode`   | Stripped down version of our 420coin client implementation that only takes part in the network node discovery protocol, but does not run any of the higher level application protocols. It can be used as a lightweight bootstrap node to aid in finding peers in private networks.                                                                                                                                                                                                                                                                 |
|     `evm`     | Developer utility version of the EVM (420coin Virtual Machine) that is capable of running bytecode snippets within a configurable environment and execution mode. Its purpose is to allow isolated, fine-grained debugging of EVM opcodes (e.g. `evm --code 60ff60ff --debug run`).                                                                                                                                                                                                                                                                     |
| `g420rpctest` | Developer utility tool to support our [420coin/rpc-test](https://github.com/420coin/rpc-tests) test suite which validates baseline conformity to the [420coin JSON RPC](https://github.com/420coin/wiki/wiki/JSON-RPC) specs. Please see the [test suite's readme](https://github.com/420coin/rpc-tests/blob/master/README.md) for details.                                                                                                                                                                                                     |
|   `rlpdump`   | Developer utility tool to convert binary RLP ([Recursive Length Prefix](https://github.com/420coin/wiki/wiki/RLP)) dumps (data encoding used by the 420coin protocol both network as well as consensus wise) to user-friendlier hierarchical representation (e.g. `rlpdump --hex CE0183FFFFFFC4C304050583616263`).                                                                                                                                                                                                                                 |
|   `puppeth`   | a CLI wizard that aids in creating a new 420coin network.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                           |

## Running `g420`

Going through all the possible command line flags is out of scope here (please consult our
[CLI Wiki page](https://github.com/420integrated/go-420coin/wiki/Command-Line-Options)),
but we've enumerated a few common parameter combos to get you up to speed quickly
on how you can run your own `g420` instance.

### Full node on the main 420coin network

By far the most common scenario is people wanting to simply interact with the 420coin
network: create accounts; transfer funds; deploy and interact with contracts. For this
particular use-case the user doesn't care about years-old historical data, so we can
fast-sync quickly to the current state of the network. To do so:

```shell
$ g420 console
```

This command will:
 * Start `g420` in fast sync mode (default, can be changed with the `--syncmode` flag),
   causing it to download more data in exchange for avoiding processing the entire history
   of the 420coin network, which is very CPU intensive.
 * Start up `g420`'s built-in interactive [JavaScript console](https://github.com/420integrated/go-420coin/wiki/JavaScript-Console),
   (via the trailing `console` subcommand) through which you can invoke all official [`web3` methods](https://github.com/420coin/wiki/wiki/JavaScript-API)
   as well as `g420`'s own [management APIs](https://github.com/420integrated/go-420coin/wiki/Management-APIs).
   This tool is optional and if you leave it out you can always attach to an already running
   `g420` instance with `g420 attach`.

### Configuration

As an alternative to passing the numerous flags to the `g420` binary, you can also pass a
configuration file via:

```shell
$ g420 --config /path/to/your_config.toml
```

To get an idea how the file should look like you can use the `dumpconfig` subcommand to
export your existing configuration:

```shell
$ g420 --your-favourite-flags dumpconfig
```

*Note: This works only with `g420` v1.6.0 and above.*

### Programmatically interfacing `g420` nodes

As a developer, sooner rather than later you'll want to start interacting with `g420` and the
420coin network via your own programs and not manually through the console. To aid
this, `g420` has built-in support for a JSON-RPC based APIs (standard APIs such as JSON-RPC
and `g420` specific APIs such as Management-APIs).
These can be exposed via HTTP, WebSockets and IPC (UNIX sockets on UNIX based
platforms, and named pipes on Windows).

The IPC interface is enabled by default and exposes all the APIs supported by `g420`,
whereas the HTTP and WS interfaces need to manually be enabled and only expose a
subset of APIs due to security reasons. These can be turned on/off and configured as
you'd expect.

HTTP based JSON-RPC API options:

  * `--http` Enable the HTTP-RPC server
  * `--http.addr` HTTP-RPC server listening interface (default: `localhost`)
  * `--http.port` HTTP-RPC server listening port (default: `6174`)
  * `--http.api` API's offered over the HTTP-RPC interface (default: `420,net,web3`)
  * `--http.corsdomain` Comma separated list of domains from which to accept cross origin requests (browser enforced)
  * `--ws` Enable the WS-RPC server
  * `--ws.addr` WS-RPC server listening interface (default: `localhost`)
  * `--ws.port` WS-RPC server listening port (default: `8546`)
  * `--ws.api` API's offered over the WS-RPC interface (default: `420,net,web3`)
  * `--ws.origins` Origins from which to accept websockets requests
  * `--ipcdisable` Disable the IPC-RPC server
  * `--ipcapi` API's offered over the IPC-RPC interface (default: `admin,debug,420,miner,net,personal,shh,txpool,web3`)
  * `--ipcpath` Filename for IPC socket/pipe within the datadir (explicit paths escape it)

You'll need to use your own programming environments' capabilities (libraries, tools, etc) to
connect via HTTP, WS or IPC to a `g420` node configured with the above flags and you'll
need to speak [JSON-RPC](https://www.jsonrpc.org/specification) on all transports. You
can reuse the same connection for multiple requests!

**Note: Please understand the security implications of opening up an HTTP/WS based
transport before doing so! Hackers on the internet are actively trying to subvert
420coin nodes with exposed APIs! Further, all browser tabs can access locally
running web servers, so malicious web pages could try to subvert locally available
APIs!**

### Operating a private network

Maintaining your own private network is more involved as a lot of configurations taken for
granted in the official networks need to be manually set up.

#### Defining the private genesis state

First, you'll need to create the genesis state of your networks, which all nodes need to be
aware of and agree upon. This consists of a small JSON file (e.g. call it `genesis.json`):

```json
{
  "config": {
    "chainId": <arbitrary positive integer>,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0
  },
  "alloc": {},
  "coinbase": "0x0000000000000000000000000000000000000000",
  "difficulty": "0x20000",
  "extraData": "",
  "smokeLimit": "0x2fefd8",
  "nonce": "0x0000000000000042",
  "mixhash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "parentHash": "0x0000000000000000000000000000000000000000000000000000000000000000",
  "timestamp": "0x00"
}
```

The above fields should be fine for most purposes, although we'd recommend changing
the `nonce` to some random value so you prevent unknown remote nodes from being able
to connect to you. If you'd like to pre-fund some accounts for easier testing, create
the accounts and populate the `alloc` field with their addresses.

```json
"alloc": {
  "0x0000000000000000000000000000000000000001": {
    "balance": "111111111"
  },
  "0x0000000000000000000000000000000000000002": {
    "balance": "222222222"
  }
}
```

With the genesis state defined in the above JSON file, you'll need to initialize **every**
`g420` node with it prior to starting it up to ensure all blockchain parameters are correctly
set:

```shell
$ g420 init path/to/genesis.json
```

#### Creating the rendezvous point

With all nodes that you want to run initialized to the desired genesis state, you'll need to
start a bootstrap node that others can use to find each other in your network and/or over
the internet. The clean way is to configure and run a dedicated bootnode:

```shell
$ bootnode --genkey=boot.key
$ bootnode --nodekey=boot.key
```

With the bootnode online, it will display an [`enode` URL](https://github.com/420coin/wiki/wiki/enode-url-format)
that other nodes can use to connect to it and exchange peer information. Make sure to
replace the displayed IP address information (most probably `[::]`) with your externally
accessible IP to get the actual `enode` URL.

*Note: You could also use a full-fledged `g420` node as a bootnode, but it's the less
recommended way.*

#### Starting up your member nodes

With the bootnode operational and externally reachable (you can try
`telnet <ip> <port>` to ensure it's indeed reachable), start every subsequent `g420`
node pointed to the bootnode for peer discovery via the `--bootnodes` flag. It will
probably also be desirable to keep the data directory of your private network separated, so
do also specify a custom `--datadir` flag.

```shell
$ g420 --datadir=path/to/custom/data/folder --bootnodes=<bootnode-enode-url-from-above>
```

*Note: Since your network will be completely cut off from the main and test networks, you'll
also need to configure a miner to process transactions and create new blocks for you.*

#### Running a private miner

Mining on the public 420coin network is a complex task as it's only feasible using GPUs,
requiring an OpenCL or CUDA enabled `ethminer` instance. For information on such a
setup, please consult the [420coinMining subreddit](https://www.reddit.com/r/420coinMining/)
and the [ethminer](https://github.com/420coin-mining/ethminer) repository.

In a private network setting, however a single CPU miner instance is more than enough for
practical purposes as it can produce a stable stream of blocks at the correct intervals
without needing heavy resources (consider running on a single thread, no need for multiple
ones either). To start a `g420` instance for mining, run it with all your usual flags, extended
by:

```shell
$ g420 <usual-flags> --mine --miner.threads=1 --fourtwentycoinbase=0x0000000000000000000000000000000000000000
```

Which will start mining blocks and transactions on a single CPU thread, crediting all
proceedings to the account specified by `--fourtwentycoinbase`. You can further tune the mining
by changing the default smoke limit blocks converge to (`--targetsmokelimit`) and the price
transactions are accepted at (`--smokeprice`).

## Contribution

Thank you for considering to help out with the source code! We welcome contributions
from anyone on the internet, and are grateful for even the smallest of fixes!

If you'd like to contribute to go-420coin, please fork, fix, commit and send a pull request
for the maintainers to review and merge into the main code base. If you wish to submit
more complex changes though, please check up with the core devs first on [our gitter channel](https://gitter.im/420coin/go-420coin)
to ensure those changes are in line with the general philosophy of the project and/or get
some early feedback which can make both your efforts much lighter as well as our review
and merge procedures quick and simple.

Please make sure your contributions adhere to our coding guidelines:

 * Code must adhere to the official Go [formatting](https://golang.org/doc/effective_go.html#formatting)
   guidelines (i.e. uses [gofmt](https://golang.org/cmd/gofmt/)).
 * Code must be documented adhering to the official Go [commentary](https://golang.org/doc/effective_go.html#commentary)
   guidelines.
 * Pull requests need to be based on and opened against the `master` branch.
 * Commit messages should be prefixed with the package(s) they modify.
   * E.g. "420, rpc: make trace configs optional"

Please see the [Developers' Guide](https://github.com/420integrated/go-420coin/wiki/Developers'-Guide)
for more details on configuring your environment, managing project dependencies, and
testing procedures.

## License

The go-420coin library (i.e. all code outside of the `cmd` directory) is licensed under the
[GNU Lesser General Public License v3.0](https://www.gnu.org/licenses/lgpl-3.0.en.html),
also included in our repository in the `COPYING.LESSER` file.

The go-420coin binaries (i.e. all code inside of the `cmd` directory) is licensed under the
[GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html), also
included in our repository in the `COPYING` file.
