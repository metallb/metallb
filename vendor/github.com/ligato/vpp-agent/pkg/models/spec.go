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

package models

import (
	"fmt"
	"net"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/gogo/protobuf/proto"
	api "github.com/ligato/vpp-agent/api/genericmanager"
)

var (
	validModule = regexp.MustCompile(`^[-a-z0-9_]+(?:\.[-a-z0-9_]+)?$`)
	validType   = regexp.MustCompile(`^[-a-z0-9_]+$`)
)

// Spec defines model specification used for registering model.
type Spec api.Model

type registeredModel struct {
	Spec

	protoName string
	keyPrefix string
	modelPath string

	modelOptions
}

type modelOptions struct {
	nameTemplate string
	nameFunc     NameFunc
}

// ModelOption defines function type which sets model options.
type ModelOption func(*modelOptions)

// WithNameTemplate returns option for models which sets function
// for generating name of instances using custom template.
func WithNameTemplate(t string) ModelOption {
	return func(opts *modelOptions) {
		opts.nameFunc = NameTemplate(t)
		opts.nameTemplate = t
	}
}

// ProtoName returns proto message name registered with the model.
func (m registeredModel) ProtoName() string {
	return m.protoName
}

// Path returns path for the model.
func (m registeredModel) Path() string {
	return m.modelPath
}

// KeyPrefix returns key prefix for the model.
func (m registeredModel) KeyPrefix() string {
	return m.keyPrefix
}

// ParseKey parses the given key and returns item name
// or returns empty name and valid as false if the key is not valid.
func (m registeredModel) ParseKey(key string) (name string, valid bool) {
	name = strings.TrimPrefix(key, m.keyPrefix)
	if name == key || name == "" {
		name = strings.TrimPrefix(key, m.modelPath)
	}
	if name != key && name != "" {
		// TODO: validate name?
		return name, true
	}
	return "", false
}

// IsKeyValid returns true if given key is valid for this model.
func (m registeredModel) IsKeyValid(key string) bool {
	_, valid := m.ParseKey(key)
	return valid
}

// StripKeyPrefix returns key with prefix stripped.
func (m registeredModel) StripKeyPrefix(key string) string {
	if name, valid := m.ParseKey(key); valid {
		return name
	}
	return key
}

var (
	registeredModels = make(map[string]*registeredModel)
	modelPaths       = make(map[string]string)

	debugRegister = strings.Contains(os.Getenv("DEBUG_MODELS"), "register")
)

// Register registers the protobuf message with given model specification.
func Register(pb proto.Message, spec Spec, opts ...ModelOption) *registeredModel {
	model := &registeredModel{
		Spec:      spec,
		protoName: proto.MessageName(pb),
	}

	// Check duplicate registration
	if _, ok := registeredModels[model.protoName]; ok {
		panic(fmt.Sprintf("proto message %q already registered", model.protoName))
	}

	// Validate model spec
	if !validModule.MatchString(spec.Module) {
		panic(fmt.Sprintf("module for model %s is invalid", model.protoName))
	}
	if !validType.MatchString(spec.Type) {
		panic(fmt.Sprintf("model type for %s is invalid", model.protoName))
	}
	if !strings.HasPrefix(spec.Version, "v") {
		panic(fmt.Sprintf("model version for %s is invalid", model.protoName))
	}

	// Generate keys & paths
	model.modelPath = buildModelPath(spec.Version, spec.Module, spec.Type)
	if pn, ok := modelPaths[model.modelPath]; ok {
		panic(fmt.Sprintf("path prefix %q already used by: %s", model.modelPath, pn))
	}
	modulePath := strings.Replace(spec.Module, ".", "/", -1)
	model.keyPrefix = fmt.Sprintf("config/%s/%s/%s/", modulePath, spec.Version, spec.Type)

	// Use GetName as fallback for generating name
	if _, ok := pb.(named); ok {
		model.nameFunc = func(obj interface{}) (s string, e error) {
			return obj.(named).GetName(), nil
		}
	}

	// Apply custom options
	for _, opt := range opts {
		opt(&model.modelOptions)
	}

	registeredModels[model.protoName] = model
	modelPaths[model.modelPath] = model.protoName

	if debugRegister {
		fmt.Printf("- registered model: %+v\t%q\n", model, model.modelPath)
	}

	return model
}

func buildModelPath(version, module, typ string) string {
	return fmt.Sprintf("%s.%s.%s", module, version, typ)
}

type named interface {
	GetName() string
}

// NameFunc represents function which can name model instance.
type NameFunc func(obj interface{}) (string, error)

func NameTemplate(t string) NameFunc {
	tmpl := template.Must(
		template.New("name").Funcs(funcMap).Option("missingkey=error").Parse(t),
	)
	return func(obj interface{}) (string, error) {
		var s strings.Builder
		if err := tmpl.Execute(&s, obj); err != nil {
			return "", err
		}
		return s.String(), nil
	}
}

var funcMap = template.FuncMap{
	"ipnet": func(s string) map[string]interface{} {
		_, ipNet, _ := net.ParseCIDR(s)
		maskSize, _ := ipNet.Mask.Size()
		return map[string]interface{}{
			"IP":       ipNet.IP.String(),
			"MaskSize": maskSize,
		}
	},
}
