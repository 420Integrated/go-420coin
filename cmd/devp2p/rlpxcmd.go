// Copyright 2020 420integrated
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
	"fmt"
	"net"

	"github.com/420integrated/go-420coin/cmd/devp2p/internal/ethtest"
	"github.com/420integrated/go-420coin/crypto"
	"github.com/420integrated/go-420coin/p2p"
	"github.com/420integrated/go-420coin/p2p/rlpx"
	"github.com/420integrated/go-420coin/rlp"
	"gopkg.in/urfave/cli.v1"
)

var (
	rlpxCommand = cli.Command{
		Name:  "rlpx",
		Usage: "RLPx Commands",
		Subcommands: []cli.Command{
			rlpxPingCommand,
			rlpxFourtwentyTestCommand,
		},
	}
	rlpxPingCommand = cli.Command{
		Name:   "ping",
		Usage:  "ping <node>",
		Action: rlpxPing,
	}
	rlpxFourtwentyTestCommand = cli.Command{
		Name:      "fourtwenty-test",
		Usage:     "Runs tests against a node",
		ArgsUsage: "<node> <path_to_chain.rlp_file>",
		Action:    rlpxFourtwentyTest,
		Flags:     []cli.Flag{testPatternFlag},
	}
)

func rlpxPing(ctx *cli.Context) error {
	n := getNodeArg(ctx)
	fd, err := net.Dial("tcp", fmt.Sprintf("%v:%d", n.IP(), n.TCP()))
	if err != nil {
		return err
	}
	conn := rlpx.NewConn(fd, n.Pubkey())
	ourKey, _ := crypto.GenerateKey()
	_, err = conn.Handshake(ourKey)
	if err != nil {
		return err
	}
	code, data, _, err := conn.Read()
	if err != nil {
		return err
	}
	switch code {
	case 0:
		var h fourtwentytest.Hello
		if err := rlp.DecodeBytes(data, &h); err != nil {
			return fmt.Errorf("invalid handshake: %v", err)
		}
		fmt.Printf("%+v\n", h)
	case 1:
		var msg []p2p.DiscReason
		if rlp.DecodeBytes(data, &msg); len(msg) == 0 {
			return fmt.Errorf("invalid disconnect message")
		}
		return fmt.Errorf("received disconnect message: %v", msg[0])
	default:
		return fmt.Errorf("invalid message code %d, expected handshake (code zero)", code)
	}
	return nil
}

func rlpxFourtwentyTest(ctx *cli.Context) error {
	if ctx.NArg() < 3 {
		exit("missing path to chain.rlp as command-line argument")
	}

	suite := fourtwentytest.NewSuite(getNodeArg(ctx), ctx.Args()[1], ctx.Args()[2])

	// Filter and run test cases.
	tests := suite.AllTests()
	if ctx.IsSet(testPatternFlag.Name) {
		tests = utesting.MatchTests(tests, ctx.String(testPatternFlag.Name))
	}
	results := utesting.RunTests(tests, os.Stdout)
	if fails := utesting.CountFailures(results); fails > 0 {
		return fmt.Errorf("%v of %v tests passed.", len(tests)-fails, len(tests))
	}
	fmt.Printf("all tests passed\n")
	return nil
}
