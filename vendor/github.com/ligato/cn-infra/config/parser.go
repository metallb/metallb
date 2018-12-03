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

package config

import (
	"io/ioutil"
	"os"

	"github.com/ghodss/yaml"
)

// ParseConfigFromYamlFile parses a configuration from a file in YAML
// format. The file's location is specified by the <path> parameter and the
// resulting config is stored into the structure referenced by the <cfg>
// parameter.
// If the file doesn't exist or cannot be read, the returned error will
// be of type os.PathError. An untyped error is returned in case the file
// doesn't contain a valid YAML configuration.
func ParseConfigFromYamlFile(path string, cfg interface{}) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(b, cfg)
	if err != nil {
		return err
	}
	return nil
}

// SaveConfigToYamlFile saves the configuration <cfg> into a YAML-formatted file
// at the location <path> with permissions defined by <perm>.
// <comment>, if non-empty, is printed at the beginning of the file before
// the configuration is printed (with a line break in between). Each line in <comment>
// should thus begin with the number sign ( # ).
// If the file cannot be created af the location, os.PathError is returned.
// An untyped error is returned if the configuration couldn't be marshaled
// into the YAML format.
func SaveConfigToYamlFile(cfg interface{}, path string, perm os.FileMode, comment string) error {
	bytes, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if comment != "" {
		bytes = append([]byte(comment+"\n"), bytes...)
	}

	err = ioutil.WriteFile(path, bytes, perm)
	if err != nil {
		return err
	}
	return nil
}
