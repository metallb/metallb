//  Copyright (c) 2018 Cisco and/or its affiliates.
//
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at:
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package agent_test

import (
	"testing"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/infra"
	. "github.com/onsi/gomega"
)

func TestDescendantPluginsNoDep(t *testing.T) {
	RegisterTestingT(t)
	plugin := &PluginNoDeps{}
	agent := agent.NewAgent(agent.AllPlugins(plugin))
	Expect(agent).ToNot(BeNil())
	Expect(agent.Options()).ToNot(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(1))
	Expect(agent.Options().Plugins[0]).To(Equal(plugin))
}

func TestDescendantPluginsOneLevelDep(t *testing.T) {
	RegisterTestingT(t)

	plugin := &PluginOneDep{}
	plugin.SetName("OneDep")
	agent := agent.NewAgent(agent.AllPlugins(plugin))
	Expect(agent).ToNot(BeNil())
	Expect(agent.Options()).ToNot(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(2))
	Expect(agent.Options().Plugins[0]).To(Equal(&plugin.Plugin2))
	Expect(agent.Options().Plugins[1]).To(Equal(plugin))
}

func TestDescendantPluginsTwoLevelsDeep(t *testing.T) {
	RegisterTestingT(t)
	plugin := &PluginTwoLevelDeps{}
	plugin.SetName("TwoDep")
	plugin.PluginTwoLevelDep1.SetName("Dep1")
	plugin.PluginTwoLevelDep2.SetName("Dep2")
	agent := agent.NewAgent(agent.AllPlugins(plugin))
	Expect(agent).ToNot(BeNil())
	Expect(agent.Options()).ToNot(BeNil())
	Expect(agent.Options().Plugins).ToNot(BeNil())
	Expect(len(agent.Options().Plugins)).To(Equal(4))
	Expect(agent.Options().Plugins[0]).To(Equal(&plugin.PluginTwoLevelDep1.Plugin2))
	Expect(agent.Options().Plugins[1]).To(Equal(&plugin.PluginTwoLevelDep1))
	Expect(agent.Options().Plugins[2]).To(Equal(&plugin.PluginTwoLevelDep2))
	Expect(agent.Options().Plugins[3]).To(Equal(plugin))

}

// Various Test Structs after this point

// PluginNoDeps contains no plugins.
type PluginNoDeps struct {
	infra.PluginName
	Plugin1 MissignCloseMethod
	Plugin2 struct {
		Dep1B string
	}
}

func (p *PluginNoDeps) Init() error  { return nil }
func (p *PluginNoDeps) Close() error { return nil }

// PluginOneDep contains one plugin (another is missing Close method).
type PluginOneDep struct {
	infra.PluginName
	Plugin1 MissignCloseMethod
	Plugin2 TestPlugin
}

func (p *PluginOneDep) Init() error  { return nil }
func (p *PluginOneDep) Close() error { return nil }

type PluginTwoLevelDeps struct {
	infra.PluginName
	PluginTwoLevelDep1 PluginOneDep
	PluginTwoLevelDep2 TestPlugin
}

func (p *PluginTwoLevelDeps) Init() error  { return nil }
func (p *PluginTwoLevelDeps) Close() error { return nil }

// MissignCloseMethod implements only Init() but not Close() method.
type MissignCloseMethod struct {
}

// Init does nothing.
func (*MissignCloseMethod) Init() error {
	return nil
}
