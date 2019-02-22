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
	"path"

	"github.com/gogo/protobuf/proto"
)

// Key is a shorthand for the GetKey for avoid error checking.
func Key(x proto.Message) string {
	key, err := GetKey(x)
	if err != nil {
		panic(err)
	}
	return key
}

// Name is a shorthand for the GetName for avoid error checking.
func Name(x proto.Message) string {
	name, err := GetName(x)
	if err != nil {
		panic(err)
	}
	return name
}

// Path is a shorthand for the GetPath for avoid error checking.
func Path(x proto.Message) string {
	path, err := GetPath(x)
	if err != nil {
		panic(err)
	}
	return path
}

// Model returns registered model for the given proto message.
func Model(x proto.Message) registeredModel {
	model, err := GetModel(x)
	if err != nil {
		panic(err)
	}
	return model
}

// GetKey returns complete key for gived model,
// including key prefix defined by model specification.
// It returns error if given model is not registered.
func GetKey(x proto.Message) (string, error) {
	model, err := GetModel(x)
	if err != nil {
		return "", err
	}
	name, err := model.nameFunc(x)
	if err != nil {
		return "", err
	}
	key := path.Join(model.keyPrefix, name)
	return key, nil
}

// GetName
func GetName(x proto.Message) (string, error) {
	model, err := GetModel(x)
	if err != nil {
		return "", err
	}
	name, err := model.nameFunc(x)
	if err != nil {
		return "", err
	}
	return name, nil
}

// GetKeyPrefix returns key prefix for gived model.
// It returns error if given model is not registered.
func GetPath(x proto.Message) (string, error) {
	model, err := GetModel(x)
	if err != nil {
		return "", err
	}
	return model.Path(), nil
}

// GetModel returns registered model for the given proto message.
func GetModel(x proto.Message) (registeredModel, error) {
	protoName := proto.MessageName(x)
	model, found := registeredModels[protoName]
	if !found {
		return registeredModel{}, fmt.Errorf("no model registered for %s", protoName)
	}
	return *model, nil
}
