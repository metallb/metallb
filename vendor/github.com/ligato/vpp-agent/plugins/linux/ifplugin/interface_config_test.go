// Copyright (c) 2018 Cisco and/or its affiliates.
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

package ifplugin_test

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/go-errors/errors"
	"github.com/vishvananda/netlink"

	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
	"github.com/ligato/vpp-agent/idxvpp/nametoidx"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin"
	"github.com/ligato/vpp-agent/plugins/linux/ifplugin/ifaceidx"
	"github.com/ligato/vpp-agent/plugins/linux/model/interfaces"
	"github.com/ligato/vpp-agent/plugins/linux/nsplugin"
	"github.com/ligato/vpp-agent/tests/linuxmock"
	. "github.com/onsi/gomega"
)

/* Linux interface configurator init and close */

// Test init function
func TestLinuxInterfaceConfiguratorInit(t *testing.T) {
	plugin, _, _, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)
	// Base fields
	Expect(plugin).ToNot(BeNil())
	Expect(msChan).ToNot(BeNil())
	// Mappings & cache
	Expect(plugin.GetLinuxInterfaceIndexes()).ToNot(BeNil())
	Expect(plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetInterfaceByNameCache()).ToNot(BeNil())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveLen(0))
	Expect(plugin.GetInterfaceByMsCache()).ToNot(BeNil())
	Expect(plugin.GetInterfaceByMsCache()).To(HaveLen(0))
}

/* Linux interface configurator test cases */

// Configure simple Veth without peer
func TestLinuxConfiguratorAddSingleVeth(t *testing.T) {
	plugin, _, _, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	data := getVethInterface("veth1", "peer1", 1)
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
}

// Configure Veth with missing data
func TestLinuxConfiguratorAddSingleVethWithoutData(t *testing.T) {
	plugin, _, _, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	data := getVethInterface("veth1", "peer1", 1)
	data.HostIfName = ""
	data.Veth = nil
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).Should(HaveOccurred())
}

// Configure simple Veth with peer
func TestLinuxConfiguratorAddVethPair(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v2",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 1,
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 2,
	})
	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 0))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 0))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
	// Verify registration
	_, meta, found := plugin.GetLinuxInterfaceIndexes().LookupIdx("veth1")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, meta, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth2")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Verify interface by name config
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
	// Verify interface by microservice cache
	Expect(plugin.GetInterfaceByNameCache()).ToNot(HaveKeyWithValue("veth1-ms", BeNil()))
	Expect(plugin.GetInterfaceByNameCache()).ToNot(HaveKeyWithValue("veth1-ms", BeNil()))
}

// Configure simple Veth with peer in microservice-type namespace
func TestLinuxConfiguratorAddVethPairInMicroserviceNs(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v2",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 1,
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 2,
	})
	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
	// Verify registration
	_, meta, found := plugin.GetLinuxInterfaceIndexes().LookupIdx("veth1")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	_, meta, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth2")
	Expect(found).To(BeTrue())
	Expect(meta).ToNot(BeNil())
	// Verify interface by name config
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
	// Verify interface by microservice cache
	ms, ok := plugin.GetInterfaceByMsCache()["veth1-ms"]
	Expect(ok).To(BeTrue())
	Expect(ms).To(HaveLen(1))
	ms, ok = plugin.GetInterfaceByMsCache()["veth2-ms"]
	Expect(ok).To(BeTrue())
	Expect(ms).To(HaveLen(1))
}

// Configure simple Veth with peer while Veth ns is not available
func TestLinuxConfiguratorAddVethPairVethNsNotAvailable(t *testing.T) {
	plugin, _, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(false)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	data, found := plugin.GetInterfaceByNameCache()["veth1"]
	Expect(found).To(BeTrue())
	Expect(data).ToNot(BeNil())
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred())
	data, found = plugin.GetInterfaceByNameCache()["veth2"]
	Expect(found).To(BeTrue())
	Expect(data).ToNot(BeNil())
	// Verify registration
	_, _, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth1")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth2")
	Expect(found).To(BeFalse())
	// Verify interface by name config
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while peer ns is not available
func TestLinuxConfiguratorAddVethPairPeerNsNotAvailable(t *testing.T) {
	plugin, _, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(false)

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	data, found := plugin.GetInterfaceByNameCache()["veth1"]
	Expect(found).To(BeTrue())
	Expect(data).ToNot(BeNil())
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred())
	data, found = plugin.GetInterfaceByNameCache()["veth2"]
	Expect(found).To(BeTrue())
	Expect(data).ToNot(BeNil())
	// Verify registration
	_, _, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth1")
	Expect(found).To(BeFalse())
	_, _, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("veth2")
	Expect(found).To(BeFalse())
	// Verify interface by name config
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while switching ns returns error
func TestLinuxConfiguratorAddVethPairSwitchNsError(t *testing.T) {
	plugin, _, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("SwitchNamespace").ThenReturn(fmt.Errorf("switch-namespace error"))

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	data, found := plugin.GetInterfaceByNameCache()["veth1"]
	Expect(found).To(BeTrue())
	Expect(data).ToNot(BeNil())
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred())
}

// Configure simple Veth with peer while peer ns is not available
func TestLinuxConfiguratorAddVethPairPeerSwitchToNsWhileRemovingObsoleteErr(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("SwitchToNamespace").ThenReturn(fmt.Errorf("remove-obsolete-1-err"))
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v2",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 1,
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 2,
	})

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth which needs to be removed. Covers all 4 cases.
func TestLinuxConfiguratorAddVethPairPeerRemoveObsolete(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	nsMock.When("ConvertMicroserviceNsToPidNs").ThenReturn(&nsplugin.Namespace{
		Type: nsplugin.PidRefNs,
	},
	)
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: "v2",
		},
	})
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-ms-obsolete-peer")
	// Second obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth2-ms-obsolete-peer")
	// Third obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-cfg-obsolete-peer")
	// Fourth obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth2-cfg-obsolete-peer")
	// Complete
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 1,
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 2,
	})

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth - interface exists error
func TestLinuxConfiguratorAddVethPairPeerRemoveObsoleteIfExistsError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(fmt.Errorf("interface-exists-err"))

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred()) // Expect error
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth - interface type error
func TestLinuxConfiguratorAddVethPairPeerRemoveObsoleteIfTypeError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn(fmt.Errorf("interface-type-err"))

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred()) // Expect error
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth - interface type does not match error
func TestLinuxConfiguratorAddVethPairPeerRemoveObsoleteIfTypeMatchError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-ms-obsolete-peer")
	// Second obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("tap") // instead of veth

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred()) // Expect error
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth - get peer name error
func TestLinuxConfiguratorAddVethPairPeerRemoveObsoleteGetPeerNameError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-ms-obsolete-peer")
	// Second obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth2-ms-obsolete-peer")
	// Third obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn(fmt.Errorf("get-veth-peer-err"))

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred()) // Expect error
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer while there is an obsolete veth - delete obsotele interface error
func TestLinuxConfiguratorAddVethPairPeerRemoveObsoleteDeletePeerNameError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// First obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-ms-obsolete-peer")
	ifMock.When("DelVethInterfacePair").ThenReturn()
	// Second obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth2-ms-obsolete-peer")
	ifMock.When("DelVethInterfacePair").ThenReturn()
	// Third obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-cfg-obsolete-peer")
	ifMock.When("DelVethInterfacePair").ThenReturn()
	// Fourth obsolete veth removal
	ifMock.When("InterfaceExists").ThenReturn(true)
	ifMock.When("GetInterfaceType").ThenReturn("veth")
	ifMock.When("GetVethPeerName").ThenReturn("veth1-ms-obsolete-peer")
	ifMock.When("DelVethInterfacePair").ThenReturn(fmt.Errorf("del-veth-interface-pair-error"))

	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 1))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 1))
	Expect(err).Should(HaveOccurred()) // Expect error
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth2", Not(BeNil())))
}

// Configure simple Veth with peer - add veth pair error
func TestLinuxConfiguratorAddVethPairError(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	ifMock.When("AddVethInterfacePair").ThenReturn(fmt.Errorf("add-veth-interface-pair-err"))
	// Configure first veth
	err := plugin.ConfigureLinuxInterface(getVethInterface("veth1", "veth2", 0))
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetInterfaceByNameCache()).To(HaveKeyWithValue("veth1", Not(BeNil())))
	// Configure second veth
	err = plugin.ConfigureLinuxInterface(getVethInterface("veth2", "veth1", 0))
	Expect(err).Should(HaveOccurred())
}

// Configure Tap with hostIfName
func TestLinuxConfiguratorAddTap_TempIfName(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// Link is searched for twice
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 10,
	})

	data := getTapInterface("tap1", "", "TempIfName")
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()).To(HaveLen(1))
	// Verify registration
	_, metadata, found := plugin.GetLinuxInterfaceIndexes().LookupIdx("tap1")
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
}

// Configure Tap with hostIfName
func TestLinuxConfiguratorAddTap_HostIfName(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifnotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifnotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// Link is searched for twice
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 10,
	})

	data := getTapInterface("tap1", "HostIfName", "")
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()).To(HaveLen(1))
	// Verify registration
	_, metadata, found := plugin.GetLinuxInterfaceIndexes().LookupIdx("tap1")
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
}

// Configure linux tap where interface is registered but does not exist yet. Then use event to start
// configuration again
func TestLinuxConfiguratorAddTapTempIfNameWithWatcher(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifNotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifNotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	ifMock.When("GetLinkByName").ThenReturn(errors.New(ifplugin.LinkNotFoundErr))
	// Link is searched for twice
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	// Watcher
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 10,
	})

	data := getTapInterface("tap1", "", "TempIfName")
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()).To(HaveLen(1))
	Expect(plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()).To(HaveLen(0))
	// Verify interface registration and empty cache
	_, metadata, found := plugin.GetCachedLinuxIfIndexes().LookupIdx("TempIfName")
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
	// Give some time to watcher to start
	time.Sleep(500 * time.Millisecond)
	// Send event that interface appeared in linux
	ifNotif <- ifplugin.NewLinuxInterfaceStateNotification("tun", netlink.OperUp, &netlink.LinkAttrs{
		Name: "TempIfName",
	})
	Eventually(func() []string {
		return plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()
	}, 2).Should(HaveLen(0))
	Eventually(func() []string {
		return plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()
	}, 2).Should(HaveLen(1))
	Eventually(func() bool {
		_, _, found = plugin.GetLinuxInterfaceIndexes().LookupIdx("tap1")
		return found
	}, 2).Should(BeTrue())
}

// Configure linux tap where interface was created but then an event about its removal arrived.
func TestLinuxConfiguratorRemoveTapHostIfNameWithWatcher(t *testing.T) {
	plugin, ifMock, nsMock, _, msChan, msNotif, ifNotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifNotif)

	// Linux/namespace calls
	nsMock.When("IsNamespaceAvailable").ThenReturn(true)
	// Link is searched for twice
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name: "t1",
		},
	})
	ifMock.When("GetInterfaceByName").ThenReturn(&net.Interface{
		Index: 10,
	})

	data := getTapInterface("tap1", "", "TempIfName")
	err := plugin.ConfigureLinuxInterface(data)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()).To(HaveLen(0))
	Expect(plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()).To(HaveLen(1))
	// Verify cache registration
	_, metadata, found := plugin.GetLinuxInterfaceIndexes().LookupIdx("tap1")
	Expect(found).To(BeTrue())
	Expect(metadata).ToNot(BeNil())
	// Give some time to watcher to start
	time.Sleep(500 * time.Millisecond)
	// Send event that interface appeared in linux
	ifNotif <- ifplugin.NewLinuxInterfaceStateNotification("tun", netlink.OperDown, &netlink.LinkAttrs{
		Name: "tap1",
	})
	Eventually(func() []string {
		return plugin.GetLinuxInterfaceIndexes().GetMapping().ListNames()
	}, 2).Should(HaveLen(0))
	Eventually(func() []string {
		return plugin.GetCachedLinuxIfIndexes().GetMapping().ListNames()
	}, 2).Should(HaveLen(1))
	Eventually(func() bool {
		_, _, found = plugin.GetCachedLinuxIfIndexes().LookupIdx("tap1")
		return found
	}, 2).Should(BeTrue())
}

/* Namespace handler Test Setup */

func TestSetInterfaceNamespace(t *testing.T) {
	plugin, ifMock, _, sysMock, msChan, msNotif, ifNotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifNotif)

	// IP address list
	var ipAddresses []netlink.Addr
	ipAddresses = append(ipAddresses,
		netlink.Addr{IPNet: getIPNetAddress("10.0.0.1/24")},
		netlink.Addr{IPNet: getIPNetAddress("172.168.0.1/24")},
		netlink.Addr{IPNet: getIPNetAddress("192.168.0.1/24")},
		// Link local address which should be skipped
		netlink.Addr{IPNet: getIPNetAddress("fe80::883f:c3ff:fe9e:fba/64")})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name:  "if1",
			Flags: net.FlagUp,
		},
	})
	ifMock.When("GetAddressList").ThenReturn(ipAddresses)
	sysMock.When("LinkSetNsFd").ThenReturn()

	// Context and namespace
	ctx := nsplugin.NewNamespaceMgmtCtx()
	ns := &interfaces.LinuxInterfaces_Interface_Namespace{
		Type: interfaces.LinuxInterfaces_Interface_Namespace_NAMED_NS,
	}

	err := ifplugin.SetInterfaceNamespace(plugin, ctx, "if1", ns)
	Expect(err).To(BeNil())

	// Check calls to ensure that only required IP addresses were configured
	num, calls := ifMock.GetCallsFor("AddInterfaceIP")
	Expect(num).To(Equal(3))
	Expect(calls).ToNot(BeNil())
	for callIdx, call := range calls {
		ifName := call[0].(string)
		Expect(ifName).To(Equal("if1"))
		ipAdd := call[1].(*net.IPNet)
		if callIdx == 1 {
			Expect(ipAdd.String()).To(Equal("10.0.0.1/24"))
		}
		if callIdx == 2 {
			Expect(ipAdd.String()).To(Equal("172.168.0.1/24"))
		}
		if callIdx == 3 {
			Expect(ipAdd.String()).To(Equal("192.168.0.1/24"))
		}
	}
}

func TestSetInterfaceNamespaceIPv6(t *testing.T) {
	plugin, ifMock, _, sysMock, msChan, msNotif, ifNotif := ifTestSetup(t)
	defer ifTestTeardown(plugin, msChan, msNotif, ifNotif)

	// IP address list
	var ipAddresses []netlink.Addr
	ipAddresses = append(ipAddresses,
		netlink.Addr{IPNet: getIPNetAddress("10.0.0.1/24")},
		netlink.Addr{IPNet: getIPNetAddress("172.168.0.1/24")},
		// Link local address should not be skipped if there is another non-link-local IPv6
		netlink.Addr{IPNet: getIPNetAddress("fe80::883f:c3ff:fe9e:fba/64")},
		netlink.Addr{IPNet: getIPNetAddress("ad48::42:e8ff:feb1:e976/64")})
	ifMock.When("GetLinkByName").ThenReturn(&netlink.Tuntap{
		LinkAttrs: netlink.LinkAttrs{
			Name:  "if1",
			Flags: net.FlagUp,
		},
	})
	ifMock.When("GetAddressList").ThenReturn(ipAddresses)
	sysMock.When("LinkSetNsFd").ThenReturn()

	// Context and namespace
	ctx := nsplugin.NewNamespaceMgmtCtx()
	ns := &interfaces.LinuxInterfaces_Interface_Namespace{
		Type: interfaces.LinuxInterfaces_Interface_Namespace_NAMED_NS,
	}

	err := ifplugin.SetInterfaceNamespace(plugin, ctx, "if1", ns)
	Expect(err).To(BeNil())

	// Check calls to ensure that only required IP addresses were configured
	num, calls := ifMock.GetCallsFor("AddInterfaceIP")
	Expect(num).To(Equal(4))
	Expect(calls).ToNot(BeNil())
	for callIdx, call := range calls {
		ifName := call[0].(string)
		Expect(ifName).To(Equal("if1"))
		ipAdd := call[1].(*net.IPNet)
		if callIdx == 1 {
			Expect(ipAdd.String()).To(Equal("10.0.0.1/24"))
		}
		if callIdx == 2 {
			Expect(ipAdd.String()).To(Equal("172.168.0.1/24"))
		}
		if callIdx == 3 {
			Expect(ipAdd.String()).To(Equal("fe80::883f:c3ff:fe9e:fba/64"))
		}
		if callIdx == 4 {
			Expect(ipAdd.String()).To(Equal("ad48::42:e8ff:feb1:e976/64"))
		}
	}
}

// Todo

/* Interface Test Setup */

func ifTestSetup(t *testing.T) (*ifplugin.LinuxInterfaceConfigurator, *linuxmock.IfNetlinkHandlerMock, *linuxmock.NamespacePluginMock,
	*linuxmock.SystemMock, chan *nsplugin.MicroserviceCtx, chan *nsplugin.MicroserviceEvent, chan *ifplugin.LinuxInterfaceStateNotification) {
	RegisterTestingT(t)

	// Loggers
	pluginLog := logging.ForPlugin("linux-if-log")
	pluginLog.SetLevel(logging.DebugLevel)
	nsHandleLog := logging.ForPlugin("ns-handle-log")
	nsHandleLog.SetLevel(logging.DebugLevel)
	// Linux interface indexes
	swIfIndexes := ifaceidx.NewLinuxIfIndex(nametoidx.NewNameToIdx(pluginLog, "if", nil))
	msChan := make(chan *nsplugin.MicroserviceCtx, 100)
	ifMicroserviceNotif := make(chan *nsplugin.MicroserviceEvent, 100)
	ifNotif := make(chan *ifplugin.LinuxInterfaceStateNotification, 100)
	// Configurator
	plugin := &ifplugin.LinuxInterfaceConfigurator{}
	linuxMock := linuxmock.NewIfNetlinkHandlerMock()
	nsMock := linuxmock.NewNamespacePluginMock()
	sysMock := linuxmock.NewSystemMock()
	err := plugin.Init(pluginLog, linuxMock, nsMock, sysMock, swIfIndexes, ifMicroserviceNotif, ifNotif)
	Expect(err).To(BeNil())

	return plugin, linuxMock, nsMock, sysMock, msChan, ifMicroserviceNotif, ifNotif
}

func ifTestTeardown(plugin *ifplugin.LinuxInterfaceConfigurator,
	msChan chan *nsplugin.MicroserviceCtx, msNotif chan *nsplugin.MicroserviceEvent, ifNotif chan *ifplugin.LinuxInterfaceStateNotification) {
	err := plugin.Close()
	Expect(err).To(BeNil())
	err = safeclose.Close(msNotif, msChan, ifNotif)
	Expect(err).To(BeNil())
	logging.DefaultRegistry.ClearRegistry()
}

/* Linux interface Test Data */

func getVethInterface(ifName, peerName string, namespaceType interfaces.LinuxInterfaces_Interface_Namespace_NamespaceType) *interfaces.LinuxInterfaces_Interface {
	return &interfaces.LinuxInterfaces_Interface{
		Name:       ifName,
		Enabled:    true,
		HostIfName: ifName + "-host",
		Type:       interfaces.LinuxInterfaces_VETH,
		Namespace: func(namespaceType interfaces.LinuxInterfaces_Interface_Namespace_NamespaceType) *interfaces.LinuxInterfaces_Interface_Namespace {
			if namespaceType < 4 {
				return &interfaces.LinuxInterfaces_Interface_Namespace{
					Type:         namespaceType,
					Microservice: ifName + "-ms",
				}
			}
			return nil
		}(namespaceType),
		Veth: &interfaces.LinuxInterfaces_Interface_Veth{
			PeerIfName: peerName,
		},
	}
}

func getTapInterface(ifName, hostIfName string, tempIfName string) *interfaces.LinuxInterfaces_Interface {
	return &interfaces.LinuxInterfaces_Interface{
		Name:        ifName,
		Enabled:     true,
		Type:        interfaces.LinuxInterfaces_AUTO_TAP,
		PhysAddress: "12:E4:0E:D5:BC:DC",
		IpAddresses: []string{
			"192.168.20.3/24",
		},
		HostIfName: hostIfName,
		Tap: &interfaces.LinuxInterfaces_Interface_Tap{
			TempIfName: tempIfName,
		},
	}
}

func getIPNetAddress(ipAddr string) *net.IPNet {
	ip, ipNet, err := net.ParseCIDR(ipAddr)
	ipNet.IP = ip
	Expect(err).To(BeNil())
	return ipNet
}
