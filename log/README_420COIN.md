This package is a fork of https://github.com/ethereum/go-ethereum, with some
minor modifications required by the go-420coin codebase:

 * Support for log level `trace`
 * Modified behavior to exit on `critical` failure
 * Considerable changes to the reward structure through the consensus package to provide protocol-integrated support architecture for veterans by introducing the Veterans Fund, which receives 12.5% of the block reward and smoke with each block sealed.
