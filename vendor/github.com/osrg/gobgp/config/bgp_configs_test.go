// Copyright (C) 2016 Nippon Telegraph and Telephone Corporation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"bufio"
	"os"
	"path"
	"runtime"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestEqual(t *testing.T) {
	assert := assert.New(t)
	p1 := Prefix{
		IpPrefix:        "192.168.0.0",
		MasklengthRange: "24..32",
	}
	p2 := Prefix{
		IpPrefix:        "192.168.0.0",
		MasklengthRange: "24..32",
	}
	assert.True(p1.Equal(&p2))
	assert.False(p1.Equal(nil))
	var p3 *Prefix
	assert.False(p3.Equal(&p1))
	p3 = &Prefix{
		IpPrefix:        "192.168.0.0",
		MasklengthRange: "24..32",
	}
	assert.True(p3.Equal(&p1))
	p3.IpPrefix = "10.10.0.0"
	assert.False(p3.Equal(&p1))
	ps1 := PrefixSet{
		PrefixSetName: "ps",
		PrefixList:    []Prefix{p1, p2},
	}
	ps2 := PrefixSet{
		PrefixSetName: "ps",
		PrefixList:    []Prefix{p2, p1},
	}
	assert.True(ps1.Equal(&ps2))
	ps2.PrefixSetName = "ps2"
	assert.False(ps1.Equal(&ps2))
}

func extractTomlFromMarkdown(fileMd string, fileToml string) error {
	fMd, err := os.Open(fileMd)
	if err != nil {
		return err
	}
	defer fMd.Close()

	fToml, err := os.Create(fileToml)
	if err != nil {
		return err
	}
	defer fToml.Close()

	isBody := false
	scanner := bufio.NewScanner(fMd)
	fTomlWriter := bufio.NewWriter(fToml)
	for scanner.Scan() {
		if curText := scanner.Text(); strings.HasPrefix(curText, "```toml") {
			isBody = true
		} else if strings.HasPrefix(curText, "```") {
			isBody = false
		} else if isBody {
			if _, err := fTomlWriter.WriteString(curText + "\n"); err != nil {
				return err
			}
		}
	}

	fTomlWriter.Flush()
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func TestConfigExample(t *testing.T) {
	assert := assert.New(t)

	_, f, _, _ := runtime.Caller(0)
	fileMd := path.Join(path.Dir(f), "../docs/sources/configuration.md")
	fileToml := "/tmp/gobgpd.example.toml"
	assert.NoError(extractTomlFromMarkdown(fileMd, fileToml))
	defer os.Remove(fileToml)

	format := detectConfigFileType(fileToml, "")
	c := &BgpConfigSet{}
	v := viper.New()
	v.SetConfigFile(fileToml)
	v.SetConfigType(format)
	assert.NoError(v.ReadInConfig())
	assert.NoError(v.UnmarshalExact(c))
	assert.NoError(setDefaultConfigValuesWithViper(v, c))
}
