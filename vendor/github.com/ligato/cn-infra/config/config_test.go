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

package config_test

import (
	"testing"

	"github.com/ligato/cn-infra/config"
	. "github.com/onsi/gomega"
)

const (
	pluginWithConfigFileName = "configfileplugin"
)

func TestForPluginWithConfigFile(t *testing.T) {
	RegisterTestingT(t)
	flagDefault := pluginWithConfigFileName + ".conf"
	pluginConfig := config.ForPlugin(pluginWithConfigFileName)
	Expect(pluginConfig).ShouldNot(BeNil())

	config.DefineFlagsFor(pluginWithConfigFileName)
	Expect(pluginConfig.GetConfigName()).Should(BeEquivalentTo(flagDefault))
}

func TestForPluginWithoutConfigFile(t *testing.T) {
	RegisterTestingT(t)
	pluginName := "confignofileplugin"
	pluginConfig := config.ForPlugin(pluginName)
	Expect(pluginConfig).ShouldNot(BeNil())

	config.DefineFlagsFor(pluginName)
	configName := pluginConfig.GetConfigName()
	Expect(configName).Should(BeEquivalentTo(""))
}

func TestForPluginWithSpecifiedConfigFile(t *testing.T) {
	RegisterTestingT(t)
	pluginName := "confignofileplugin2"
	configFileName := pluginWithConfigFileName + ".conf"
	pluginConfig := config.ForPlugin(pluginName, config.WithCustomizedFlag(
		config.FlagName(pluginName), configFileName, "customized config filename"))
	Expect(pluginConfig).ShouldNot(BeNil())

	config.DefineFlagsFor(pluginName)
	configName := pluginConfig.GetConfigName()
	Expect(configName).Should(BeEquivalentTo(configFileName))
}
