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

import (
	"github.com/ligato/vpp-agent/cmd/agentctl/impl"
	"github.com/ligato/vpp-agent/cmd/agentctl/utils"
	"github.com/spf13/cobra"
)

// 'putBridgeDomain' command can be used to put bridge domain configuration to etcd. This command can be used
// with attribute flags (forward, learn, etc.) in order to change BD attributes
// and it can also manipulate inner BD configuration (attach or remove interfaces, add or remove
// ARP table or FIB table entries).
var putBridgeDomainCommand = &cobra.Command{
	Use:        "bridge-domain -l <agent-label> -n <bd-name>",
	Aliases:    []string{"b", "br", "bd"},
	SuggestFor: []string{"br", "bri", "brid", "bridg", "bridge"},
	Short:      "Create or update a vpp bridge domain.",
	Long: `
Creates a new vpp bridge domain or updates an existing one. The bridge
domain command uses several flags to set up its attributes (flood,
forward, learn, etc.) and it is also used to create or remove additional
elements related to the bridge domain: interfaces attached to the bridge
domain, ARP table entries, and FIB table entries (see help in a
specific command).

The command represents the 'C' and 'U' in the bridge
domain CrUd operations.

The new configuration is merged with the existing (old) configuration.

The bridge domain name and the vpp agent label has to be specified for each
put bridge domain operation, as shown in the examples.
`,
	Example: `
$ agentctl put bridge-domain --label <agent-label> --name <bd-name>
$ agentctl put bridge-domain -l <agent-label> -n <bd-name>
$ agentctl p b -l <agent-label> -n <bd-name>
`,
	Run: func(cmd *cobra.Command, args []string) {
		impl.CreateUpdateBridgeDomain(globalFlags.Endpoints, globalFlags.Label)
	},
}

// 'delBridgeDomain' command removes bridge domain including ARP table and all attached interfaces.
// Note: FIB table is displayed outside of the removed bridge domain, so all related
// FIB entries will stay in the config.
var delBridgeDomainCommand = &cobra.Command{
	Use:        "bridge-domain -l <agent-label> -n <bd-name>",
	Aliases:    []string{"b", "br", "bd"},
	SuggestFor: []string{"br", "bri", "brid", "bridg", "bridge"},
	Short:      "Delete a vpp bridge domain ('D' in bridge domain cruD).",
	Long: `
Removes bridge domain from the configuration including all attached interfaces,
the ARP table. This command represents the 'F' in bridge domain cruD operations.

The bridge domain name and the vpp agent label must be specified for each
delete bridge domain operation, as shown in the examples.
`,
	Example: `
$ agentctl delete bridge-domain --label <agent-label> --name <bd-name>
$ agentctl delete bridge-domain -l <agent-label> -n <bd-name>
$ agentctl d b -l <agent-label> -n <bd-name>
`,
	Run: func(cmd *cobra.Command, args []string) {
		impl.DeleteBridgeDomain(globalFlags.Endpoints, globalFlags.Label)
	},
}

// 'l2Interface' command is used to add or remove interface in bridge domain.
var l2InterfaceCommand = &cobra.Command{
	Use:        "interface -l <agent-label> -n <bd-name> -i<interface-name>",
	Aliases:    []string{"i", "if", "int"},
	SuggestFor: []string{"i", "in", "int", "inte", "interf"},
	Short:      "Attach interface to bridge domain",
	Long: `
This command is used to put interface to the bridge domain configuration.
Required flags are vpp label, bridge domain name (to identify where the interface
should be added) and an interface name as an identification.

Other configuration options allow setting split horizon group number and whether
the interface is a BVI or not.

This command is also used to remove the interface from bridge domain. To do so, use flag
"-D". Vpp label, bridge domain name and the interface name are still mandatory.
`,
	Example: `
To create an interface:
$ agentctl put bridge-domain interface --label <agent-label> --name <bd-name> --interface-name <interface-name>
$ agentctl put bridge-domain interface -l <agent-label> -n <bd-name> --interface-name <interface-name>
$ agentctl p b i -l <agent-label> -n <bd-name> --interface-name <interface-name>
To remove an interface:
$ agentctl put bridge-domain interface --label <agent-label> --name <bd-name> --interface-name <interface-name> -D
$ agentctl put bridge-domain interface -l <agent-label> -n <bd-name> --interface-name <interface-name> -D
$ agentctl p b i -l <agent-label> -n <bd-name> -i <interface-name> -D
`,
	Run: func(cmd *cobra.Command, args []string) {
		if l2InterfaceFlags.IsDelete {
			impl.DeleteInterfaceFromBridgeDomain(globalFlags.Endpoints, globalFlags.Label, &l2InterfaceFlags)
			return
		}
		impl.AddUpdateInterfaceToBridgeDomain(globalFlags.Endpoints, globalFlags.Label, &l2InterfaceFlags)
	},
}

// 'l2ArpEntry' command is used to add or remove ARP table entry in bridge domain.
var l2ArpEntryCommand = &cobra.Command{
	Use:        "arp -l <agent-label> -n <bd-name> -i<ip-address>",
	Aliases:    []string{"a", "ar"},
	SuggestFor: []string{"a", "ar"},
	Short:      "Add ARP entry to a bridge domain ARP table",
	Long: `
This command is used to add ARP entry to the ARP table within specified bridge domain.
Minimally required flags are vpp label, bridge domain name (to identify where the
interface should be added) and an IP address to identify ARP entry.

This command is also used to remove ARP entry from the table belonging to a bridge
domain. To do so, use flag "-D". Vpp label, bridge domain name and the IP address
to identify ARP entry are still mandatory.
`,
	Example: `
To create an ARP entry:
$ agentctl put bridge-domain arp --label <agent-label> --name <bd-name> --ip-address <arp-ip>
$ agentctl put bridge-domain arp -l <agent-label> -n <bd-name> --ip-address <arp-ip>
$ agentctl p b a -l <agent-label> -n <bd-name> --ip-address <arp-ip>
To remove an ARP entry:
$ agentctl put bridge-domain arp --label <agent-label> --name <bd-name> --ip-address <arp-ip> -D
$ agentctl put bridge-domain arp -l <agent-label> -n <bd-name> --ip-address <arp-ip> -D
$ agentctl p b a -l <agent-label> -n <bd-name> -i <arp-ip> -D
`,
	Run: func(cmd *cobra.Command, args []string) {
		if l2ArpEntryFlags.IsDelete {
			impl.DeleteArpEntry(globalFlags.Endpoints, globalFlags.Label, &l2ArpEntryFlags)
			return
		}
		impl.AddUpdateArpEntry(globalFlags.Endpoints, globalFlags.Label, &l2ArpEntryFlags)
	},
}

// 'l2FibEntry' command is used to add or remove FIB entry in the FIB table.
var l2FibEntryCommand = &cobra.Command{
	Use:        "fib -l <agent-label> -n <bd-name> -i<mac-address>",
	Aliases:    []string{"f", "fi"},
	SuggestFor: []string{"f", "fi"},
	Short:      "Add an entry to the FIB table",
	Long: `
This command is used to add FIB entry to the FIB table. Minimal required
flags are vpp label, bridge domain this FIB entry belongs to and a MAC
address which serves as an identification.

The fib command is also used to remove FIB entry from the table. To do so,
use flag "-D". Vpp label, bridge domain name and the MAC address are still
mandatory.
`,
	Example: `
To create a FIB entry:
$ agentctl put bridge-domain fib --label <agent-label> --name <bd-name> --physical-address <mac-address> --interface-name <outgoing-if-name>
$ agentctl put bridge-domain fib -l <agent-label> -n <bd-name> --physical-address <mac-address> --interface-name <outgoing-if-name>
$ agentctl p b f -l <agent-label> -n <bd-name> -i <mac-address> --interface-name <outgoing-if-name>
To remove a FIB entry:
$ agentctl put bridge-domain fib --label <agent-label> --name <bd-name> --physical-address <mac-address> -D
$ agentctl put bridge-domain fib -l <agent-label> -n <bd-name> --physical-address <mac-address> -D
$ agentctl p b f -l <agent-label> -n <bd-name> -i <mac-address> -D
`,
	Run: func(cmd *cobra.Command, args []string) {
		if l2FibEntryFlags.IsDelete {
			impl.DelFibEntry(globalFlags.Endpoints, globalFlags.Label, &l2FibEntryFlags)
			return
		}
		impl.AddFibEntry(globalFlags.Endpoints, globalFlags.Label, &l2FibEntryFlags)
	},
}

var (
	l2InterfaceFlags impl.BridgeDomainInterfaceFields
	l2ArpEntryFlags  impl.BridgeDomainArpFields
	l2FibEntryFlags  impl.L2FIBEntryFields
)

func init() {
	// Attach put-bridge-domain command to 'put'
	putCommand.AddCommand(putBridgeDomainCommand)
	impl.AddBridgeDomainFlags(putBridgeDomainCommand)

	// Attach delete-bridge-domain command to 'delete'
	deleteCommand.AddCommand(delBridgeDomainCommand)
	impl.AddBridgeDomainNameFlag(delBridgeDomainCommand)

	// Initialize flags for bridge domain interface command
	putBridgeDomainCommand.AddCommand(l2InterfaceCommand)
	l2InterfaceCommand.Flags().StringVarP(&l2InterfaceFlags.BdName, utils.BDName, "n", "",
		"Bridge domain name the interface belongs to")
	l2InterfaceCommand.Flags().StringVarP(&l2InterfaceFlags.IfName, utils.IfName, "i", "",
		"Name of the interface (identification)")
	l2InterfaceCommand.Flags().BoolVarP(&l2InterfaceFlags.Bvi, utils.BVI, "", false,
		"Mark interface as BVI")
	l2InterfaceCommand.Flags().Uint32VarP(&l2InterfaceFlags.SplitHorizonGroup, utils.SHZ, "", 0,
		"Set split horizon group")
	l2InterfaceCommand.Flags().BoolVarP(&l2InterfaceFlags.IsDelete, utils.IsDelete, "D", false,
		"Delete interface")

	// Initialize flags for bridge domain arp command
	putBridgeDomainCommand.AddCommand(l2ArpEntryCommand)
	l2ArpEntryCommand.Flags().StringVarP(&l2ArpEntryFlags.BdName, utils.BDName, "n", "",
		"Bridge domain name to identify where the ARP should be added")
	l2ArpEntryCommand.Flags().StringVarP(&l2ArpEntryFlags.IPAddress, utils.IPAddress, "i", "",
		"IP address of the ARP table entry (identification)")
	l2ArpEntryCommand.Flags().StringVarP(&l2ArpEntryFlags.PhysAddress, utils.PhysAddress, "", "",
		"Physical address of the ARP table entry")
	l2ArpEntryCommand.Flags().BoolVarP(&l2ArpEntryFlags.IsDelete, utils.IsDelete, "D", false,
		"Delete ARP entry")

	// Initialize flags for bridge domain fib command
	putBridgeDomainCommand.AddCommand(l2FibEntryCommand)
	l2FibEntryCommand.Flags().StringVarP(&l2FibEntryFlags.BdName, utils.BDName, "n", "",
		"Bridge domain name to identify where the FIB should be added")
	l2FibEntryCommand.Flags().StringVarP(&l2FibEntryFlags.PhysAddress, utils.PhysAddress, "i", "",
		"Unique destination MAC address (identification)")
	l2FibEntryCommand.Flags().Uint8VarP(&l2FibEntryFlags.Action, utils.IsDrop, "", 0,
		"Matching frame action (0 - Forward, 1 - Drop)")
	l2FibEntryCommand.Flags().StringVarP(&l2FibEntryFlags.OutgoingInterface, utils.IfName, "", "",
		"Outgoing interface for matching frames")
	l2FibEntryCommand.Flags().BoolVarP(&l2FibEntryFlags.StaticConfig, utils.StaticConfig, "", false,
		"True if this is a statically configured FIB entry")
	l2FibEntryCommand.Flags().BoolVarP(&l2FibEntryFlags.BVI, utils.BVI, "", false,
		"True if the MAC address is a bridge virtual interface MAC")
	l2FibEntryCommand.Flags().BoolVarP(&l2FibEntryFlags.IsDelete, utils.IsDelete, "D", false,
		"Delete FIB entry")
}
