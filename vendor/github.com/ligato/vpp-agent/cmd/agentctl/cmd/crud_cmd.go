// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import "github.com/spf13/cobra"

// Root command 'put' can be used to create vpp configuration elements. The command uses no labels
// (except the global ones), so the following command is required.
var putCommand = &cobra.Command{
	Use:     "put",
	Aliases: []string{"p"},
	Short:   "Create or update vpp configuration ('C' and 'U' in CrUd).",
	Long: "Put vpp configuration attributes (Interfaces, Bridge Domains, L2," +
		" X-Connects, Routes).",
}

// Root command 'delete' can be used to remove vpp configuration elements. The command uses no labels
// (except the global ones), so the following command is required
var deleteCommand = &cobra.Command{
	Use:     "delete",
	Aliases: []string{"d"},
	Short:   "Delete vpp configuration ('D' in cruD).",
	Long: "Remove vpp configuration attributes (Interfaces, Bridge Domains, L2," +
		"X-Connects, Routes).",
}

func init() {
	RootCmd.AddCommand(putCommand)
	RootCmd.AddCommand(deleteCommand)
}
