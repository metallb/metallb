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
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/spf13/cobra"
)

// 'put interface' command can be used to add interface configuration to etcd. It uses
// several following commands (according to interface type).
var putInterfaceCommand = &cobra.Command{
	Use:        "interface [command] -l <agent-label> -n <if-name>",
	Aliases:    []string{"i", "in", "int", "if"},
	SuggestFor: []string{"in", "inte", "inter", "interf", "interfa", "ifc"},
	Short:      "Create or update a vpp interface ('C' and 'U' in interface CrUd).",
	Long: `
Creates a new vpp interface or updates an existing one. This command
represents the 'C' and 'U' in interface CrUd operations.

An interface is created using one of the interface-type-specific
subcommands or from a JSON-encoded data read from a file or from the
CLI.

If the new configuration comes from JSON encoded data, the existing (old)
configuration is overwritten with the new configuration. If new
configuration is entered using a subcommand, it is merged with the
existing (old) configuration. Subcommands can be used for fine-grained
updates (patching) of individual interface attributes.

The interface name and the vpp agent label must be specified for each
put interface operation, as shown in the examples.
`,
	Example: `
$ agentctl put interface --label <agent-label> --name <interface-name>
$ agentctl put interface -l <agent-label> -n <interface-name>
$ agentctl p i -l <agent-label> -n <interface-name>
`,
}

// 'delete interface' command can be used to remove interface configuration from etcd.
var delInterfaceCommand = &cobra.Command{
	Use:        "interface -l <agent-label> -n <if-name>",
	Aliases:    []string{"i", "in", "int", "if"},
	SuggestFor: []string{"in", "inte", "inter", "interf", "interfa", "ifc"},
	Short:      "Delete a vpp interface ('D in interface cruD).",
	Long: `
Removes an interface from the configuration. This command
represents the 'D' in interface cruD operations.

Any interface type can be removed with this command.

The interface name and the vpp agent label must be specified for each
delete interface operation, as shown in the examples.
`,
	Example: `
$ agentctl delete interface --label <agent-label> --name <interface-name>
$ agentctl delete interface -l <agent-label> -n <interface-name>
$ agentctl d i -l <agent-label> -n <interface-name>
`,
	Run: func(cmd *cobra.Command, args []string) {
		impl.InterfaceDel(globalFlags.Endpoints, globalFlags.Label)
	},
}

var putAfPktCmd = &cobra.Command{
	Use:     "af-packet",
	Aliases: []string{"a", "af", "af-pkt"},
	Short:   "Create/update AF_PACKET_INTERFACE.",
	Long:    "If configuration exists, it is merged with the new configuration",

	Run: func(cmd *cobra.Command, args []string) {
		impl.PutAfPkt(globalFlags.Endpoints, globalFlags.Label, &afPktCmdFlags)
	},
}

var putEthernetCmd = &cobra.Command{
	Use:     "ethernet",
	Aliases: []string{"e", "et", "eth", "ethe", "ether"},
	Short:   "Create/update ETHERNET_CSMACD interface.",
	Long:    "If configuration exists, it is merged with the new configuration",
	Run: func(cmd *cobra.Command, args []string) {
		impl.PutEthernet(globalFlags.Endpoints, globalFlags.Label)
	},
}

var putLoopbackCmd = &cobra.Command{
	Use:     "loopback",
	Aliases: []string{"l", "lo", "loo", "loop", "loopb", "loopbk"},
	Short:   "Create/update SOFTWARE_LOOPBACK interface",
	Long:    "If configuration exists, it is merged with the new configuration",
	Run: func(cmd *cobra.Command, args []string) {
		impl.PutLoopback(globalFlags.Endpoints, globalFlags.Label)
	},
}

var putMemifCmd = &cobra.Command{
	Use:     "memif",
	Aliases: []string{"m", "me", "mem"},
	Short:   "Create/update MEMORY_INTERFACE.",
	Long:    "If configuration exists, it is merged with the new configuration",
	Run: func(cmd *cobra.Command, args []string) {
		impl.PutMemif(globalFlags.Endpoints, globalFlags.Label, &memifCmdFlags)
	},
}

var putTapCmd = &cobra.Command{
	Use:     "tap",
	Aliases: []string{"t"},
	Short:   "Create/update TAP_INTERFACE.",
	Long: `
Creates a new vpp interface or updates an existing one. This command
represents the 'C' and 'U' in interface CrUd operations.

flags define interface attributes configurable via this command.

$ agentctl put interface --label <agent-label> --name <interface-name>
$ agentctl put interface -l <agent-label> -n <interface-name>
$ agentctl p i -l <agent-label> -n <interface-name>
`,
	Run: func(cmd *cobra.Command, args []string) {
		impl.PutTap(globalFlags.Endpoints, globalFlags.Label)
	},
}

var putVxLanCmd = &cobra.Command{
	Use:     "vxlan",
	Aliases: []string{"v", "vx", "vxl"},
	Short:   "Create/update VXLAN_TUNNEL interface.",
	Long:    "If configuration exists, it is merged with the new configuration",
	Run: func(cmd *cobra.Command, args []string) {
		impl.PutVxLan(globalFlags.Endpoints, globalFlags.Label, &vxLanFlags)
	},
}

var putIfJSONCmd = &cobra.Command{
	Use:     "json -l <agent-label>  << EOF\n  > <json-encoded-configuration>\n  > EOF\n",
	Aliases: []string{"j", "js"},
	Short:   "Create/update interface with JSON config entered from stdin",
	Long: `
Reads interface configuration encoded in JSON from stdin and uses it
to create a new vpp interface or overwrite the configuration  of an
existing interface with the same name. The 'Update' operation is a
replace, not a merge: the new configuration REPLACES the existing
configuration for the interface.
`,
	Run: func(cmd *cobra.Command, args []string) {
		impl.IfJSONPut(globalFlags.Endpoints, globalFlags.Label)
	},
}

var (
	interfaceCmdFlags string
	afPktCmdFlags     interfaces.Interfaces_Interface_Afpacket
	memifCmdFlags     interfaces.Interfaces_Interface_Memif
	vxLanFlags        interfaces.Interfaces_Interface_Vxlan
)

func init() {
	// Add interface command to 'put'.
	putCommand.AddCommand(putInterfaceCommand)
	putInterfaceCommand.Flags().StringVarP(&interfaceCmdFlags, "file", "f", "",
		"get configuration from file")

	// Attach all interface type sub-commands with initialized command-specific flags.
	initAfPktCmd()
	initEthernetCmd()
	initLoopbackCmd()
	initMemifCmd()
	initTapCmd()
	initVxLanCmd()
	initIfJSONCmd()

	// Add interface command to 'delete'.
	deleteCommand.AddCommand(delInterfaceCommand)
	impl.AddInterfaceNameFlag(delInterfaceCommand)
}

func initAfPktCmd() {
	putInterfaceCommand.AddCommand(putAfPktCmd)
	impl.AddCommonIfPutFlags(putAfPktCmd)
	putAfPktCmd.Flags().StringVar(&afPktCmdFlags.HostIfName, utils.HostIfName, "", "Host interface name")
}

func initEthernetCmd() {
	putInterfaceCommand.AddCommand(putEthernetCmd)
	impl.AddCommonIfPutFlags(putEthernetCmd)
}

func initLoopbackCmd() {
	putInterfaceCommand.AddCommand(putLoopbackCmd)
	impl.AddCommonIfPutFlags(putLoopbackCmd)
}

func initMemifCmd() {
	putInterfaceCommand.AddCommand(putMemifCmd)
	impl.AddCommonIfPutFlags(putMemifCmd)

	var mode uint8
	putMemifCmd.Flags().BoolVar(&memifCmdFlags.Master, utils.MemifMaster, false, "If set to 'true', this MEMIF is the link master")
	putMemifCmd.Flags().Uint8Var(&mode, utils.MemifMode, 0, "Memif operational mode (0 = ethernet, 1 = ip, 2 = punt/inject)")
	switch mode {
	case 0:
		memifCmdFlags.Mode = interfaces.Interfaces_Interface_Memif_ETHERNET
	case 1:
		memifCmdFlags.Mode = interfaces.Interfaces_Interface_Memif_IP
	case 2:
		memifCmdFlags.Mode = interfaces.Interfaces_Interface_Memif_PUNT_INJECT
	default:
		memifCmdFlags.Mode = interfaces.Interfaces_Interface_Memif_ETHERNET
	}
	putMemifCmd.Flags().Uint32Var(&memifCmdFlags.Id, utils.MemifID, 0, "Memif identifier used to match opposite sides of the connection")
	putMemifCmd.Flags().StringVar(&memifCmdFlags.SocketFilename, utils.MemifSktFileName, "", "Memif socket file name")
	putMemifCmd.Flags().StringVar(&memifCmdFlags.Secret, utils.MemifSecret, "", "Memif secret used for the authentication")
	putMemifCmd.Flags().Uint32Var(&memifCmdFlags.RingSize, utils.MemifRingSize, 0, "Memif ring buffer size (Bytes)")
	putMemifCmd.Flags().Uint32Var(&memifCmdFlags.BufferSize, utils.MemifBufferSize, 0, "Memif total buffer size (Bytes)")
	putMemifCmd.Flags().Uint32Var(&memifCmdFlags.RxQueues, utils.MemifRxQueues, 1, "The number of memif rx queues (only valid for slave)")
	putMemifCmd.Flags().Uint32Var(&memifCmdFlags.TxQueues, utils.MemifTxQueues, 1, "The number of memif tx queues (only valid for slave)")
}

func initTapCmd() {
	putInterfaceCommand.AddCommand(putTapCmd)
	impl.AddCommonIfPutFlags(putTapCmd)
}

func initVxLanCmd() {
	putInterfaceCommand.AddCommand(putVxLanCmd)
	impl.AddCommonIfPutFlags(putVxLanCmd)
	putVxLanCmd.Flags().StringVar(&vxLanFlags.SrcAddress, utils.VxLanSrcAddr, "", "VxLan source IPv4 or IPv6 address in CIDR format")
	putVxLanCmd.Flags().StringVar(&vxLanFlags.DstAddress, utils.VxLanDstAddr, "", "VxLan destination IPv4 or IPv6 address in CIDR format")
	putVxLanCmd.Flags().Uint32Var(&vxLanFlags.Vni, utils.VxLanVni, 0, "VxLan Vni")
}

func initIfJSONCmd() {
	//putIfJsonCmd.SetUsageFunc(ifputJSONUsage)
	putInterfaceCommand.AddCommand(putIfJSONCmd)

}
