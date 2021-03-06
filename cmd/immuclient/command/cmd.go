/*
Copyright 2021 CodeNotary, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package immuclient

import (
	"fmt"
	"os"
	"strings"

	"github.com/codenotary/immudb/cmd/docs/man"
	c "github.com/codenotary/immudb/cmd/helper"
	"github.com/codenotary/immudb/cmd/version"
	"github.com/spf13/cobra"
)

func Execute(cmd *cobra.Command) error {
	if isCommand(commandNames(cmd.Commands())) {
		if err := cmd.Execute(); err != nil {
			return err
		}
	}
	return nil
}

func NewCommand() *cobra.Command {
	version.App = "immuclient"
	cl := NewCommandLine()
	cmd, err := cl.NewCmd()
	if err != nil {
		c.QuitToStdErr(err)
	}

	// login and logout
	cl.Register(cmd)
	// man file generator
	cmd.AddCommand(man.Generate(cmd, "immuclient", "./cmd/docs/man/immuclient"))
	cmd.AddCommand(version.VersionCmd())
	return cmd
}

func isCommand(args []string) bool {
	if len(os.Args) > 1 {
		if strings.HasPrefix(os.Args[1], "-") {
			for i := range args {
				for j := range os.Args {
					if args[i] == os.Args[j] {
						fmt.Printf("Please sort your commands in \"immudb [command] [flags]\" order. \n")
						return true
					}
				}
			}
		}
	}
	return true
}

func commandNames(cms []*cobra.Command) []string {
	args := make([]string, 0)
	for i := range cms {
		arg := strings.Split(cms[i].Use, " ")[0]
		args = append(args, arg)
	}
	return args
}
