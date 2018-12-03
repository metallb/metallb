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
	"sort"
	"strings"

	"github.com/buger/goterm"
)

// Used as a header/info message
const (
	InPkt     string = "IN-PKT"
	InBytes   string = "IN-BYTES"
	InErrPkt  string = "IN-ERR-PKT"
	InMissPkt string = "IN-MISS-PKT"
	InNoBuf   string = "IN-NO-BUFF"
	OutPkt    string = "OUT-PKT"
	OutBytes  string = "OUT-BYTES"
	OutErrPkt string = "OUT-ERR-PKT"
	Drop      string = "DROP"
	Punt      string = "PUNT"
	Ipv4Pkt   string = "IPV4-PKT"
	Ipv6Pkt   string = "IPV6-PKT"

	NoData  string = "Nothing to show, add some data or change the filter"
	NoSpace string = "There is not enough space to show the whole table. Use --short"
)

// TableVppDataContext is the data context for one vpp (where number of interfaces == rows)
// with mandatory info: agent label (vpp name), list of interfaces used in table
// (without filtered items) and interfaceMap with statistics data.
type TableVppDataContext struct {
	vppName      string
	interfaces   []string
	interfaceMap map[string]InterfaceWithMD
}

// PrintDataAsTable receives the complete etcd data, applies all filters, flags
// and other restrictions, then prints table filled with interface statistics.
func (ed EtcdDump) PrintDataAsTable(table *goterm.Table, filter []string, short bool, active bool) (*goterm.Table, error) {
	// Resolve vpp and interface filters. Name(s) of the vpp(s) and interface(s) that will be shown are returned.
	vppFilter, interfaceFilter, err := processFilters(filter)
	if err != nil {
		return table, err
	}
	// Get all vpp keys.
	sortedVppKeys := ed.getSortedKeys()
	// Apply vpp filter, remove all unwanted vpp keys and sort.
	if len(vppFilter) > 0 {
		sortedVppKeys = filterElements(sortedVppKeys, vppFilter)
		sort.Strings(sortedVppKeys)
	}

	// Get list of table data contexts (vpp + interfaces + statistics data).
	// Every context contains only wanted interfaces at this point (interface filter is applied).
	// If an active flag is set, all full-zero rows are removed as well.
	tableDataContext, dataDisplayed := ed.getAllTableDataContexts(sortedVppKeys, interfaceFilter, active)

	// If no data are displayed, print message
	if !dataDisplayed {
		fmt.Fprintf(table, "%s", NoData)
		return table, nil
	}

	// According to the table data context, prepare list of items which will be in the table header.
	headerItems := getHeaderItems(short, active, tableDataContext)
	// Get two strings: the table header with required items and the formatting string used in print.
	tableHeader, format := composeHeader(headerItems)

	// Print table header and every row
	fmt.Fprintf(table, "%s", tableHeader)
	for _, row := range tableDataContext {
		row.printTable(table, headerItems, format)
	}

	return table, nil
}

// Returns list of TableVppDataContext objects with all necessary info to print the whole table.
// Bool flag watches whether some data will be actually displayed after applying all filters.
func (ed EtcdDump) getAllTableDataContexts(sortedVppKeys []string, interfaceFilter []string, active bool) ([]*TableVppDataContext, bool) {
	dataDisplayed := false
	var tableDataContext []*TableVppDataContext

	// Iterate over every agent label (vpp).
	for _, vppName := range sortedVppKeys {
		vppData, exists := ed[vppName]
		if exists {
			// Obtain map of all interfaces belonging to the vpp.
			interfaceMap := vppData.Interfaces
			var interfaceNames []string
			// Leverage all interface names to the separate list.
			for name := range interfaceMap {
				interfaceNames = append(interfaceNames, name)
			}

			// Remove all interfaces without statistics data todo add flag to show <no_data> in table.
			interfaceNames = filterInterfacesWithoutData(interfaceNames, interfaceMap)

			// Apply interface filter - remove all interfaces except those defined in the filter.
			if len(interfaceFilter) > 0 && len(interfaceNames) > 0 {
				interfaceNames = filterElements(interfaceNames, interfaceFilter)
			}
			// Apply active filter - remove all full-zero rows (columns are processed later).
			if active && len(interfaceNames) > 0 {
				interfaceNames = filterInactiveInterfaces(interfaceNames, interfaceMap)
			}
			sort.Strings(interfaceNames)

			// Verify there are some interfaces left. If there are data for at least
			// one interface, the data will be shown.
			if len(interfaceNames) > 0 {
				dataDisplayed = true
			}
			rowDataContext := &TableVppDataContext{vppName, interfaceNames, interfaceMap}
			tableDataContext = append(tableDataContext, rowDataContext)
		}
	}
	return tableDataContext, dataDisplayed
}

// Process provided vpp/interface filters.
func processFilters(filter []string) ([]string, []string, error) {
	var vppFilter []string
	var interfaceFilter []string
	if len(filter) == 1 {
		vppFilter = strings.Split(filter[0], ",")
	} else if len(filter) == 2 {
		// There should be only one vpp.
		vppFilter = strings.Split(filter[0], ",")
		if len(vppFilter) > 1 {
			return vppFilter, interfaceFilter,
				fmt.Errorf("Interface filter not allowed with multiple vpp: %v", vppFilter)
		}
		interfaceFilter = strings.Split(filter[1], ",")
	}
	return vppFilter, interfaceFilter, nil
}

// Remove interfaces without statistics data from the provided list and return the remaining items.
func filterInterfacesWithoutData(interfaces []string, interfaceMap map[string]InterfaceWithMD) []string {
	var filteredInterfaces []string
	for _, iface := range interfaces {
		ifaceData := interfaceMap[iface]
		if ifaceData.State == nil || ifaceData.State.InterfaceState == nil || ifaceData.State.InterfaceState.Statistics == nil {
			continue
		}
		filteredInterfaces = append(filteredInterfaces, iface)
	}
	return filteredInterfaces
}

// Remove all items which are not a part of the filter list (also the items which
// are part of the filter but don't exist).
func filterElements(itemList []string, filter []string) []string {
	var filteredItems []string
	for _, item := range itemList {
		for _, filter := range filter {
			if item == filter {
				filteredItems = append(filteredItems, item)
			}
		}
	}
	return filteredItems
}

// Remove all inactive interfaces from provided interface list.
func filterInactiveInterfaces(interfaceList []string, interfaceMap map[string]InterfaceWithMD) []string {
	var filteredInterfaces []string
	for _, name := range interfaceList {
		ifaceData := interfaceMap[name]
		if ifaceData.State == nil || ifaceData.State.InterfaceState == nil || ifaceData.State.InterfaceState.Statistics == nil {
			continue
		}
		stats := ifaceData.State.InterfaceState.Statistics
		if stats.InPackets != 0 || stats.InBytes != 0 || stats.InErrorPackets != 0 || stats.InMissPackets != 0 ||
			stats.InNobufPackets != 0 || stats.OutPackets != 0 || stats.OutBytes != 0 ||
			stats.OutErrorPackets != 0 || stats.DropPackets != 0 || stats.PuntPackets != 0 ||
			stats.Ipv4Packets != 0 || stats.Ipv6Packets != 0 {
			filteredInterfaces = append(filteredInterfaces, name)
		}
	}
	return filteredInterfaces
}

// getHeaderItems resolves every field in statistics according to the format parameters. Rules:
// * Short table shows InPackets, OutPackets and Drop.
// * Full table shows All fields except InNoBufPackets, and Ipv4Packets, and Ipv6Packets.
// * Detail table shows everything.
// * Active table shows only non-zero columns (at least one value has to be non-zero).
//   All restrictions mentioned above are applied as well.
// Other filters (vpp, interface) are already applied at this point, though data context
// does not contain these elements.
// Also all full-zero rows were removed if active flag is set, so only columns are resolved here.
func getHeaderItems(short bool, active bool, dataContext []*TableVppDataContext) []string {
	var headerItems []string
	if active && short {
		// Need to keep item order
		if isNonZeroColumn(dataContext, InPkt) {
			headerItems = append(headerItems, InPkt)
		}
		if isNonZeroColumn(dataContext, OutPkt) {
			headerItems = append(headerItems, OutPkt)
		}
		if isNonZeroColumn(dataContext, Drop) {
			headerItems = append(headerItems, Drop)
		}
	} else if active && !short {
		// Need to keep item order
		if isNonZeroColumn(dataContext, InPkt) {
			headerItems = append(headerItems, InPkt)
		}
		if isNonZeroColumn(dataContext, InBytes) {
			headerItems = append(headerItems, InBytes)
		}
		if isNonZeroColumn(dataContext, InErrPkt) {
			headerItems = append(headerItems, InErrPkt)
		}
		if isNonZeroColumn(dataContext, InMissPkt) {
			headerItems = append(headerItems, InMissPkt)
		}
		if isNonZeroColumn(dataContext, OutPkt) {
			headerItems = append(headerItems, OutPkt)
		}
		if isNonZeroColumn(dataContext, OutBytes) {
			headerItems = append(headerItems, OutBytes)
		}
		if isNonZeroColumn(dataContext, OutErrPkt) {
			headerItems = append(headerItems, OutErrPkt)
		}
		if isNonZeroColumn(dataContext, Drop) {
			headerItems = append(headerItems, Drop)
		}
		if isNonZeroColumn(dataContext, Punt) {
			headerItems = append(headerItems, Punt)
		}
	} else if short {
		// Need to keep item order
		headerItems = append(headerItems, InPkt, OutPkt, Drop)
	} else { // Full
		// Need to keep item order
		headerItems = append(headerItems, InPkt, InBytes, InErrPkt, InMissPkt, OutPkt, OutBytes, OutErrPkt, Drop, Punt)
	}
	return headerItems
}

// Compose header and formatting string for table. The formatting string will be applied to every table row.
func composeHeader(headerItems []string) (tableHeader string, format string) {
	// Base format for header (vpp and interface are always shown)
	tableHeader = "VPP\tINTERFACE"
	// Base format string (interface name is always shown, vpp is added later in printTable)
	format = "\t%s"
	for _, headerItem := range headerItems {
		// Add new tab and the header item.
		tableHeader = tableHeader + "\t" + headerItem
		// Add new tab and the digit placeholder for the item.
		format = format + "\t%d"
	}
	// Finally add new line to the end of the header and the format.
	tableHeader = tableHeader + "\n"
	format = format + "\n"

	return tableHeader, format
}

// Checks specific data type in all table rows and evaluates whether there are non-zero values.
// If particular statistic data type is zero-value in the whole column, return false, otherwise return true.
func isNonZeroColumn(dataContext []*TableVppDataContext, dataType string) bool {
	// Assume that the whole column is zero (easier to evaluate)
	isZeroColumn := true
	// Iterate over every VPP (agent label)
	for _, row := range dataContext {
		// Iterate over every interface in that vpp
		for _, iface := range row.interfaces {
			ifaceData := row.interfaceMap[iface]
			// All these interfaces have statistics
			stats := ifaceData.State.InterfaceState.Statistics
			// If particular data type contains non-zero value, evaluation is finished
			switch dataType {
			case InPkt:
				{
					if stats.InPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case InBytes:
				{
					if stats.InBytes != 0 {
						isZeroColumn = false
						break
					}
				}
			case InErrPkt:
				{
					if stats.InErrorPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case InMissPkt:
				{
					if stats.InMissPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case OutPkt:
				{
					if stats.OutPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case OutBytes:
				{
					if stats.OutBytes != 0 {
						isZeroColumn = false
						break
					}
				}
			case OutErrPkt:
				{
					if stats.OutErrorPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case Drop:
				{
					if stats.DropPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case Punt:
				{
					if stats.PuntPackets != 0 {
						isZeroColumn = false
						break
					}
				}
			case Ipv4Pkt:
				{
					if stats.Ipv4Packets != 0 {
						isZeroColumn = false
						break
					}
				}
			case Ipv6Pkt:
				{
					if stats.Ipv6Packets != 0 {
						isZeroColumn = false
						break
					}
				}
			}
		}
	}
	// At Last revert the value to return correct result.
	return !isZeroColumn
}

// Print row to the provided table object for a single vpp. todo cutoff data list to separate method
func (row *TableVppDataContext) printTable(table *goterm.Table, items []string, format string) {
	// Iterate over vpp interfaces.
	for index, name := range row.interfaces {
		ifaceData := row.interfaceMap[name]
		if ifaceData.State == nil || ifaceData.State.InterfaceState == nil || ifaceData.State.InterfaceState.Statistics == nil {
			continue
		}
		stats := ifaceData.State.InterfaceState.Statistics
		var dataList []interface{}
		// Vpp label is written only once for better readability.
		if index == 0 {
			dataList = append(dataList, row.vppName, name)
		} else {
			dataList = append(dataList, " ", name)
		}
		for _, item := range items {
			if item == InPkt {
				dataList = append(dataList, stats.InPackets)
			}
			if item == InBytes {
				dataList = append(dataList, stats.InBytes)
			}
			if item == InErrPkt {
				dataList = append(dataList, stats.InErrorPackets)
			}
			if item == InMissPkt {
				dataList = append(dataList, stats.InMissPackets)
			}
			if item == OutPkt {
				dataList = append(dataList, stats.OutPackets)
			}
			if item == OutBytes {
				dataList = append(dataList, stats.OutBytes)
			}
			if item == OutErrPkt {
				dataList = append(dataList, stats.OutErrorPackets)
			}
			if item == Drop {
				dataList = append(dataList, stats.DropPackets)
			}
			if item == Punt {
				dataList = append(dataList, stats.PuntPackets)
			}
			if item == Ipv4Pkt {
				dataList = append(dataList, stats.Ipv4Packets)
			}
			if item == Ipv6Pkt {
				dataList = append(dataList, stats.Ipv6Packets)
			}
		}

		if index == 0 {
			// Print vpp name in the first line only.
			fmt.Fprintf(table, "%s:"+format, dataList...)
		} else {
			fmt.Fprintf(table, "%s"+format, dataList...)
		}
	}
}
