// Copyright 2018 The The 420Integrated Development Group
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
	"os"
	"testing"

	"github.com/docker/docker/pkg/reexec"
	"github.com/420integrated/go-420coin/internal/cmdtest"
)

type test420key struct {
	*cmdtest.TestCmd
}

// spawns 420key with the given command line args.
func run420key(t *testing.T, args ...string) *test420key {
	tt := new(test420key)
	tt.TestCmd = cmdtest.NewTestCmd(t, tt)
	tt.Run("420key-test", args...)
	return tt
}

func TestMain(m *testing.M) {
	// Run the app if we've been exec'd as "420key-test" in run420key.
	reexec.Register("420key-test", func() {
		if err := app.Run(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	})
	// check if we have been reexec'd
	if reexec.Init() {
		return
	}
	os.Exit(m.Run())
}
