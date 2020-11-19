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
	"bufio"
	"errors"
	"fmt"
	"os"
	"reflect"
	"unicode"

	cli "gopkg.in/urfave/cli.v1"

	"github.com/420integrated/go-420coin/cmd/utils"
	"github.com/420integrated/go-420coin/420"
	"github.com/420integrated/go-420coin/internal/420api"
	"github.com/420integrated/go-420coin/log"
	"github.com/420integrated/go-420coin/node"
	"github.com/420integrated/go-420coin/params"
	"github.com/naoina/toml"
)

var (
	dumpConfigCommand = cli.Command{
		Action:      utils.MigrateFlags(dumpConfig),
		Name:        "dumpconfig",
		Usage:       "Show configuration values",
		ArgsUsage:   "",
		Flags:       append(append(nodeFlags, rpcFlags...), whisperFlags...),
		Category:    "MISCELLANEOUS COMMANDS",
		Description: `The dumpconfig command shows configuration values.`,
	}

	configFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

type fourtwentystatsConfig struct {
	URL string `toml:",omitempty"`
}

// whisper has been deprecated, but clients out there might still have [Shh]
// in their config, which will crash. Cut them some slack by keeping the
// config, and displaying a message that those config switches are ineffectual.
// To be removed circa Q1 2021 -- @gballet.
type whisperDeprecatedConfig struct {
	MaxMessageSize                        uint32  `toml:",omitempty"`
	MinimumAcceptedPOW                    float64 `toml:",omitempty"`
	RestrictConnectionBetweenLightClients bool    `toml:",omitempty"`
}

type g420Config struct {
	Fourtwenty       fourtwenty.Config
	Shh              whisperDeprecatedConfig
	Node             node.Config
	Fourtwentystats  fourtwentystatsConfig
}

func loadConfig(file string, cfg *g420Config) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

func defaultNodeConfig() node.Config {
	cfg := node.DefaultConfig
	cfg.Name = clientIdentifier
	cfg.Version = params.VersionWithCommit(gitCommit, gitDate)
	cfg.HTTPModules = append(cfg.HTTPModules, "fourtwenty")
	cfg.WSModules = append(cfg.WSModules, "fourtwenty")
	cfg.IPCPath = "g420.ipc"
	return cfg
}

// makeConfigNode loads g420 configuration and creates a blank node instance.
func makeConfigNode(ctx *cli.Context) (*node.Node, g420Config) {
	// Load defaults.
	cfg := g420Config{
		Fourtwenty:  fourtwenty.DefaultConfig,
		Node:        defaultNodeConfig(),
	}

	// Load config file.
	if file := ctx.GlobalString(configFileFlag.Name); file != "" {
		if err := loadConfig(file, &cfg); err != nil {
			utils.Fatalf("%v", err)
		}

		if cfg.Shh != (whisperDeprecatedConfig{}) {
			log.Warn("Deprecated whisper config detected. Whisper has been moved to github.com/420coin/whisper")
		}
	}

	// Apply flags.
	utils.SetNodeConfig(ctx, &cfg.Node)
	stack, err := node.New(&cfg.Node)
	if err != nil {
		utils.Fatalf("Failed to create the protocol stack: %v", err)
	}
	utils.SetFourtwentyConfig(ctx, stack, &cfg.Fourtwenty)
	if ctx.GlobalIsSet(utils.FourtwentyStatsURLFlag.Name) {
		cfg.Fourtwentystats.URL = ctx.GlobalString(utils.FourtwentyStatsURLFlag.Name)
	}
	utils.SetShhConfig(ctx, stack)

	return stack, cfg
}

// enableWhisper returns true in case one of the whisper flags is set.
func checkWhisper(ctx *cli.Context) {
	for _, flag := range whisperFlags {
		if ctx.GlobalIsSet(flag.GetName()) {
			log.Warn("deprecated whisper flag detected. Whisper has been moved to github.com/420integrated/whisper")
		}
	}
}

// makeFullNode loads g420 configuration and creates the 420coin backend.
func makeFullNode(ctx *cli.Context) (*node.Node, fourtwentyapi.Backend) {
	stack, cfg := makeConfigNode(ctx)

	backend := utils.RegisterFourtwentyService(stack, &cfg.Fourtwenty)

	checkWhisper(ctx)
	// Configure GraphQL if requested
	if ctx.GlobalIsSet(utils.GraphQLEnabledFlag.Name) {
		utils.RegisterGraphQLService(stack, backend, cfg.Node)
	}
	// Add the 420coin Stats daemon if requested.
	if cfg.Fourtwentystats.URL != "" {
		utils.RegisterFourtwentyStatsService(stack, backend, cfg.Fourtwentystats.URL)
	}
	return stack, backend
}

// dumpConfig is the dumpconfig command.
func dumpConfig(ctx *cli.Context) error {
	_, cfg := makeConfigNode(ctx)
	comment := ""

	if cfg.Fourtwenty.Genesis != nil {
		cfg.Fourtwenty.Genesis = nil
		comment += "# Note: this config doesn't contain the genesis block.\n\n"
	}

	out, err := tomlSettings.Marshal(&cfg)
	if err != nil {
		return err
	}

	dump := os.Stdout
	if ctx.NArg() > 0 {
		dump, err = os.OpenFile(ctx.Args().Get(0), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return err
		}
		defer dump.Close()
	}
	dump.WriteString(comment)
	dump.Write(out)

	return nil
}
