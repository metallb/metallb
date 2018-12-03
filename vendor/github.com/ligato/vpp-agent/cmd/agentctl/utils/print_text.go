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

package utils

import (
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"bytes"

	"github.com/ligato/cn-infra/health/statuscheck/model/status"
	"github.com/ligato/vpp-agent/plugins/vpp/model/interfaces"
	"github.com/logrusorgru/aurora.git"
)

const perLevelSpaces = 3

type pfx struct {
	printAsTree    bool
	perLevelSpaces int
}

var (
	prefixer   pfx
	treeWriter = NewTreeWriter(1, "├─", "│ ", "└─")
)

func newPrefixer(t bool, s int) pfx {
	return pfx{t, s}
}

func (p pfx) getPrefix(level int) string {
	if p.printAsTree {
		return fmt.Sprintf("%d^@%s", level, strings.Repeat(" ", level*p.perLevelSpaces))
	}
	return strings.Repeat(" ", level*p.perLevelSpaces)
}

// PrintDataAsText prints data from an EtcdDump repo in the text format. If the tree option
// is chosen, output is printed with tree lines.
func (ed EtcdDump) PrintDataAsText(showEtcd bool, printAsTree bool) (*bytes.Buffer, error) {
	prefixer = newPrefixer(printAsTree, perLevelSpaces)

	nameFuncMap := template.FuncMap{
		"setBold": setBold,
		"pfx":     getPrefix,
	}

	nameTemplate, err := template.New("name").Funcs(nameFuncMap).Parse("{{setBold .}}:\n")
	if err != nil {
		panic(err)
	}

	stsFuncMap := template.FuncMap{
		"convertTime": convertTime,
		"setBold":     setBold,
		"setRed":      setRed,
		"setOsColor":  setOsColor,
		"pfx":         getPrefix,
	}

	stsTemplate, err := template.New("status").Funcs(stsFuncMap).Parse(
		"{{pfx 1}}STATUS:" +
			"{{$etcd := .ShowEtcd}}" +
			// Iterate over status.
			"{{range $statusName, $statusData := .Status}}\n{{pfx 2}}{{$statusName}}: {{setOsColor .State}}" +
			"{{if .LastUpdate}}, Updated: {{convertTime .LastUpdate | setBold}}{{end}}" +
			"{{if .LastChange}}, Changed: {{convertTime .LastChange}}{{end}}" +
			"{{if .BuildVersion}}\n{{pfx 3}}    Version: '{{.BuildVersion}}'{{end}}" +
			"{{if .BuildDate}}, Built: {{setBold .BuildDate}}{{end}}" +
			"{{if $etcd}}\n{{pfx 3}}    ETCD: Rev {{.Rev}}, Key '{{.Key}}'{{end}}" +
			// In case there is no status
			"{{else}} {{setRed \"INACTIVE\"}}" +
			// Iterate over status - end of loop
			"{{end}}\n")
	if err != nil {
		panic(err)
	}

	ifFuncMap := template.FuncMap{
		"convertTime":    convertTime,
		"setBold":        setBold,
		"setGreen":       setGreen,
		"setRed":         setRed,
		"isEnabled":      isEnabled,
		"setStsColor":    setStsColor,
		"getIpAddresses": getIPAddresses,
		"pfx":            getPrefix,
	}

	ifTemplate, err := template.New("interfaces").Funcs(ifFuncMap).Parse(
		"{{$etcd := .ShowEtcd}}" +
			"{{$interfaceErrors := .InterfaceErrors}}" +
			"{{with .Interfaces}}\n{{pfx 1}}INTERFACES:" +

			// Iterate over interfaces.
			"{{range $ifaceName, $ifaceData := .}}\n{{pfx 2}}{{setBold $ifaceName}}" +

			// Interface overall status
			"{{with .State}}{{with .InterfaceState}} ({{.InternalName}}, ifIdx {{.IfIndex}}){{end}}{{end}}:\n{{pfx 3}}Status: <" +
			"{{with .Config}}{{with .Interface}}{{isEnabled .Enabled}}{{end}}, " +
			// 'with .Config' else
			"{{else}}{{setRed \"NOT-IN-CONFIG\"}}{{end}}, " +
			"{{with .State}}{{with .InterfaceState}}{{setStsColor \"ADMIN\" .AdminStatus}}, {{setStsColor \"OPER\" .OperStatus}}{{end}}" +
			// 'with .State' else
			"{{else}}, {{setRed \"NOT-IN-VPP\"}}{{end}}>" +

			// Interface type
			"{{with .Config}}{{with .Interface}}\n{{pfx 3}}IfType: {{.Type}}" +
			"{{end}}" +

			// IP Address and attributes (if configured)
			"{{with .Interface}}{{with .IpAddresses}}\n{{pfx 3}}IpAddr: {{getIpAddresses .}}" +
			"{{end}}{{end}}{{end}}" +

			// Physical address (if configured)
			"{{with .State}}{{with .InterfaceState}}{{if .PhysAddress}}\n{{pfx 3}}PhysAddr: {{.PhysAddress}}" +
			"{{end}}{{end}}" +

			// Link attributes (if available from state)
			"{{with .InterfaceState}}{{if or .Mtu .Speed .Duplex}}\n{{pfx 3}}LnkAtr: " +
			"{{with .Mtu}}mtu {{.}}{{end}}" +
			"{{with .Speed}}, speed {{.}}{{end}}" +
			"{{with .Duplex}}, duplex {{.}}{{end}}" +
			"{{end}}{{end}}" +

			// Interface statistics (if available from State)
			"{{with .InterfaceState}}{{with .Statistics}}" +
			"{{if or .InPackets .InBytes .OutPackets .OutBytes .Ipv4Packets .Ipv6Packets .DropPackets .InErrorPackets .InMissPackets .PuntPackets .InNobufPackets}}\n" +
			"{{pfx 3}}Stats:" +
			"\n{{pfx 4}}In: pkt {{.InPackets}}, byte {{.InBytes}}, errPkt {{.InErrorPackets}}, nobufPkt {{.InNobufPackets}}, missPkt {{.InMissPackets}}" +
			"\n{{pfx 4}}Out: pkt {{.OutPackets}}, byte {{.OutBytes}}, errPkt {{.OutErrorPackets}}" +
			"\n{{pfx 4}}Misc: drop {{.DropPackets}}, punt {{.PuntPackets}}, ipv4 {{.Ipv4Packets}}, ipv6 {{.Ipv6Packets}}" +
			"{{end}}" + // Return from 'if or'
			"{{end}}{{end}}{{end}}" +

			// Etcd metadata for both config and state records
			"{{if $etcd}}\n{{pfx 3}}ETCD:" +
			"{{with .Config}}{{with .Metadata}}\n{{pfx 4}}Cfg: Rev {{.Rev}}, Key '{{.Key}}'{{end}}{{end}}" +
			"{{with .State}}{{with .Metadata}}\n{{pfx 4}}Sts: Rev {{.Rev}}, Key '{{.Key}}'{{end}}{{end}}" +
			"{{end}}\n" +

			// Interface errors (if present)
			"{{with $interfaceErrors}}" +
			// Iterate over interfaces (Interface_errors).
			"{{range .}}{{with .InterfaceErrorList}}" +
			// Iterate over interface error list.
			"{{range .}}{{if eq .InterfaceName $ifaceName}}{{with .ErrorData}}{{pfx 3}}{{setRed \"Errors\"}}:" +
			// Iterate over error data.
			"{{range $index, $error := .}}\n{{pfx 4}}Changed: {{convertTime $error.LastChange | setBold}}, ChngType: {{$error.ChangeType}}, Msg: {{setRed $error.ErrorMessage}}" +
			// Iterate over error data - end of loop
			"{{end}}{{end}}{{end}}" +
			// Iterate over interface error list - end of loop
			"{{end}}{{end}}" +
			// Iterate over interfaces (Interface_errors) - end of loop
			"{{end}}{{end}}" +

			// Iterate over interfaces - end of loop
			"{{end}}{{end}}")
	if err != nil {
		panic(err)
	}

	bdFuncMap := template.FuncMap{
		"convertTime": convertTime,
		"setBold":     setBold,
		"setRed":      setRed,
		"pfx":         getPrefix,
	}

	bdTemplate, err := template.New("bridgeDomains").Funcs(bdFuncMap).Parse(
		"{{$etcd := .ShowEtcd}}" +
			"{{$fibTableEntries := .FibTableEntries}}" +
			"{{$bridgeDomainErrors := .BridgeDomainErrors}}" +
			"{{with .BridgeDomains}}\n{{pfx 1}}BRIDGE DOMAINS:" +

			// Iterate over bridge domains.
			"{{range $bdKey, $element := .}}\n{{pfx 2}}{{setBold $bdKey}}:" +

			// Bridge domain config attributes
			"{{with .Config}}{{with .BridgeDomain}}\n{{pfx 3}}Attributes:{{if or .Flood .UnknownUnicastFlood .Forward .Learn .ArpTermination}} <" +
			"{{if .Flood}}FLOOD{{end}}" +
			"{{if .UnknownUnicastFlood}}{{if .Flood}},{{end}} UNKN-UNICAST-FLOOD{{end}}" +
			"{{if .Forward}}{{if or .Flood .UnknownUnicastFlood}},{{end}} FORWARD{{end}}" +
			"{{if .Learn}}{{if or .Flood .UnknownUnicastFlood .Forward}},{{end}} LEARN{{end}}" +
			"{{if .ArpTermination}}{{if or .Flood .UnknownUnicastFlood .Forward .Learn}},{{end}} ARP-TERMINATION{{end}}>" +
			"{{end}}" +

			// Interface table
			"{{with .Interfaces}}\n{{pfx 3}}Interfaces:" +
			// Iterate over BD interfaces.
			"{{range $ifKey, $element := .}}\n{{pfx 4}}{{setBold $element.Name}} splitHorizonGrp {{.SplitHorizonGroup}}" +
			"{{if .BridgedVirtualInterface}}, <BVI>{{end}}" +
			// Iterate over BD interfaces - end of loop
			"{{end}}{{end}}" +

			// ARP termination table
			"{{with .ArpTerminationTable}}\n{{pfx 3}}ARP-Table:" +
			// Iterate over ARP table.
			"{{range $arpKey, $arp := .}}\n{{pfx 4}}{{$arp.IpAddress}}: {{$arp.PhysAddress}}" +
			//Iterate over ARP table - end of loop
			"{{end}}{{end}}" +
			// Out of '.Config'
			"{{end}}{{end}}" +

			// Bridge domain status
			"{{with .State}}{{with .BridgeDomainState}}" +
			"\n{{pfx 3}}Stats:" +
			"\n{{pfx 4}}Index: {{.Index}}" +
			"\n{{pfx 4}}Attributes:" +

			// Bridge domain state attributes
			"{{with .L2Params}}{{if or .Flood .UnknownUnicastFlood .Forward .Learn .ArpTermination}} <" +
			"{{if .Flood}}FLOOD{{end}}" +
			"{{if .UnknownUnicastFlood}}{{if .Flood}},{{end}} UNKN-UNICAST-FLOOD{{end}}" +
			"{{if .Forward}}{{if or .Flood .UnknownUnicastFlood}},{{end}} FORWARD{{end}}" +
			"{{if .Learn}}{{if or .Flood .UnknownUnicastFlood .Forward}},{{end}} LEARN{{end}}" +
			"{{if .ArpTermination}}{{if or .Flood .UnknownUnicastFlood .Forward .Learn}},{{end}} ARP-TERMINATION{{end}}>" +
			"{{end}}{{end}}" +
			"\n{{pfx 4}}Interfaces: ({{.InterfaceCount}}){{with .Interfaces}}" +
			// Iterate over BD status interfaces
			"{{range $ifaceIndex, $iface := .}}" +
			"\n{{pfx 5}}{{setBold $iface.Name}} shg: {{$iface.SplitHorizonGroup}}" +
			// Iterate over BD status interfaces - end of loop
			"{{end}}{{end}}" +
			"\n{{pfx 4}}BVI: <{{.BviInterface}}>" +
			// Out of '.Status"
			"{{end}}{{end}}" +

			// Etcd metadata
			"{{if $etcd}}\n{{pfx 3}}ETCD:" +
			"{{with .Config}}{{with .Metadata}}\n{{pfx 4}}Cfg: Rev {{.Rev}}, Key '{{.Key}}'{{end}}{{end}}" +
			"{{with .State}}{{with .Metadata}}\n{{pfx 4}}Sts: Rev {{.Rev}}, Key '{{.Key}}'{{end}}{{end}}" +
			"{{end}}" +

			// Bridge domain errors (if present)
			"{{with $bridgeDomainErrors}}" +
			// Iterate over bridge domains (BridgeDomain_errors).
			"{{range .}}{{with .BdErrorList}}" +
			// Iterate ove bridge domain error list.
			"{{range .}}{{if eq .BdName $element.Name}}{{with .ErrorData}}\n{{pfx 3}}{{setRed \"Errors\"}}" +
			// Iterate over error data.
			"{{range $index, $error := .}}\n{{pfx 4}}Changed: {{convertTime $error.LastChange | setBold}}, ChngType: {{$error.ChangeType}}, Msg: {{setRed $error.ErrorMessage}}" +
			// Iterate over error data - end of loop
			"{{end}}{{end}}{{end}}" +
			// Iterate ove bridge domain error list - end of loop
			"{{end}}{{end}}" +
			// Iterate over bridge domains (BridgeDomain_errors) - end of loop
			"{{end}}{{end}}" +

			// FIB table
			"\n{{with $fibTableEntries}}{{with .FibTable}}\n{{pfx 2}}FIB-Table:" +
			// Iterate over FIB table.
			"{{range $fibKey, $fib := .}}\n{{pfx 3}}{{$fib.PhysAddress}}{{with $fib.OutgoingInterface}}, {{$fib.OutgoingInterface}}{{end}}" +
			"{{with $fib.BridgeDomain}}, {{$fib.BridgeDomain}}{{end}}" +
			"{{if $fib.StaticConfig}}, <STATIC>{{end}}" +
			"{{if $fib.BridgedVirtualInterface}}, <BVI>{{end}}" +
			"{{if eq $fib.Action 0}}, <FORWARD> {{else}}, <DROP>{{end}}" +
			// Iterate over FIB table - end of loop
			"{{end}}{{end}}{{end}}\n" +

			"{{end}}{{end}}\n")

	templates := []*template.Template{}
	// Keep template order
	templates = append(templates, nameTemplate, stsTemplate, ifTemplate, bdTemplate)

	if printAsTree {
		return ed.treeRenderer(showEtcd, templates)
	}
	return ed.textRenderer(showEtcd, templates)
}

// Render data according to templates as a tree.
func (ed EtcdDump) treeRenderer(showEtcd bool, templates []*template.Template) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	for _, key := range ed.getSortedKeys() {
		treeBuffer := new(bytes.Buffer)
		vd, _ := ed[key]
		vd.ShowEtcd = showEtcd

		for _, bd := range vd.BridgeDomains {
			nl := []*string{}
			if bd.Config != nil {
				for _, bdi := range bd.Config.BridgeDomain.Interfaces {
					nl = append(nl, &bdi.Name)
				}
			}
			padRight(nl, ":")
		}

		var wasError error
		for index, templateVal := range templates {
			if index == 0 {
				// Execute first template with standard output and key.
				wasError = templateVal.Execute(os.Stdout, key)
			} else {
				wasError = templateVal.Execute(treeBuffer, vd)
			}
			if wasError != nil {
				return nil, wasError
			}
		}

		// Pass bytes written for this key to tree writer.
		treeWriter.writeBuf = treeBuffer.Bytes()
		// Render tree.
		treeWriter.FlushTree()
		fmt.Println("")
		// Add bytes to cumulative buffer (the buffer is not used to render).
		buffer.Write(treeBuffer.Bytes())
		// Reset local buffer.
		treeBuffer.Reset()
	}
	return buffer, nil
}

// Render data according to templates in text form.
func (ed EtcdDump) textRenderer(showEtcd bool, templates []*template.Template) (*bytes.Buffer, error) {
	buffer := new(bytes.Buffer)
	buffer.WriteTo(os.Stdout)
	for _, key := range ed.getSortedKeys() {
		vd, _ := ed[key]
		vd.ShowEtcd = showEtcd
		for _, bd := range vd.BridgeDomains {
			nl := []*string{}
			if bd.Config != nil {
				for _, bdi := range bd.Config.BridgeDomain.Interfaces {
					nl = append(nl, &bdi.Name)
				}
			}
			padRight(nl, ":")
		}
		var wasError error
		for index, templateVal := range templates {
			if index == 0 {
				// First with key
				wasError = templateVal.Execute(buffer, key)
			} else {
				wasError = templateVal.Execute(buffer, vd)
			}
			if wasError != nil {
				return nil, wasError
			}
		}
	}
	return buffer, nil
}

func getPrefix(level int) string {
	return prefixer.getPrefix(level)
}

func isEnabled(enabled bool) string {
	if enabled {
		return fmt.Sprintf("%s", aurora.Green("ENABLED"))
	}
	return fmt.Sprintf("%s", aurora.Red("DISABLED"))
}

func convertTime(t int64) string {
	return time.Unix(t, 0).Format("2006-01-02 15:04:05")
}
func setRed(attr interface{}) string {
	return fmt.Sprintf("%s", aurora.Red(attr))
}
func setGreen(attr interface{}) string {
	return fmt.Sprintf("%s", aurora.Green(attr))
}
func setYellow(attr interface{}) string {
	return fmt.Sprintf("%s", aurora.Brown(attr))
}
func setBold(attr interface{}) string {
	return fmt.Sprintf("%s", aurora.Bold(attr))
}

// setOsColor sets the color for the Operational State.
func setOsColor(arg status.OperationalState) string {
	switch arg {
	case status.OperationalState_OK:
		return setGreen(arg)
	case status.OperationalState_INIT:
		return setYellow(arg)
	case status.OperationalState_ERROR:
		return setRed(arg)
	default:
		return arg.String()
	}
}

func setStsColor(kind string, arg interfaces.InterfacesState_Interface_Status) string {
	sts := fmt.Sprintf("%s-%s", kind, arg)
	switch arg {
	case interfaces.InterfacesState_Interface_UP:
		return setGreen(sts)
	case interfaces.InterfacesState_Interface_DOWN:
		return setRed(sts)
	default:
		return sts
	}
}

// getIPAddresses gets a list of IPv4 addresses configured on an
// interface. The parameters are returned as a formatted string
// ready to be printed out.
func getIPAddresses(addrs []string) string {
	return strings.Join(addrs, ", ")
}
