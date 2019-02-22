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
	"reflect"

	"github.com/gogo/protobuf/proto"
	"github.com/gogo/protobuf/types"

	api "github.com/ligato/vpp-agent/api/genericmanager"
	"github.com/ligato/cn-infra/datasync"
)

// This constant is used as prefix for TypeUrl when marshalling to Any.
const ligatoModels = "models.ligato.io/"

func UnmarshalLazyValue(key string, lazy datasync.LazyValue) (proto.Message, error) {
	for _, model := range registeredModels {
		if !model.IsKeyValid(key) {
			continue
		}
		valueType := proto.MessageType(model.ProtoName())
		if valueType == nil {
			return nil, fmt.Errorf("unknown proto message defined for key %s", key)
		}
		value := reflect.New(valueType.Elem()).Interface().(proto.Message)
		// try to deserialize the value
		err := lazy.GetValue(value)
		if err != nil {
			return nil, err
		}
		return value, nil
	}
	return nil, fmt.Errorf("no model registered for key %s", key)
}

// Unmarshal is helper function for unmarshalling items.
func UnmarshalItem(m *api.Item) (proto.Message, error) {
	protoName, err := types.AnyMessageName(m.GetData().GetAny())
	if err != nil {
		return nil, err
	}
	model, found := registeredModels[protoName]
	if !found {
		return nil, fmt.Errorf("message %s is not registered as model", protoName)
	}

	itemModel := m.Id.Model
	if itemModel.Module != model.Module ||
		itemModel.Version != model.Version ||
		itemModel.Type != model.Type {
		return nil, fmt.Errorf("item model does not match the one registered (%+v)", itemModel)
	}

	var any types.DynamicAny
	if err := types.UnmarshalAny(m.GetData().GetAny(), &any); err != nil {
		return nil, err
	}
	return any.Message, nil
}

// Marshal is helper function for marshalling items.
func MarshalItem(pb proto.Message) (*api.Item, error) {
	id, err := getItemID(pb)
	if err != nil {
		return nil, err
	}

	any, err := types.MarshalAny(pb)
	if err != nil {
		return nil, err
	}
	any.TypeUrl = ligatoModels + proto.MessageName(pb)

	/*name, err := model.nameFunc(pb)
	if err != nil {
		return nil, err
	}
	path := path.Join(model.Path(), name)*/

	item := &api.Item{
		/*Ref: &api.ItemRef{
			Path: model.modelPath,
			Name: name,
		},*/
		Id: id,
		//Key: path,
		Data: &api.Data{
			Any: any,
		},
	}
	return item, nil
}

func getItemID(pb proto.Message) (*api.Item_ID, error) {
	protoName := proto.MessageName(pb)
	model, found := registeredModels[protoName]
	if !found {
		return nil, fmt.Errorf("message %s is not registered as model", protoName)
	}

	name, err := model.nameFunc(pb)
	if err != nil {
		return nil, err
	}

	return &api.Item_ID{
		Name: name,
		Model: &api.Model{
			Module:  model.Module,
			Version: model.Version,
			Type:    model.Type,
		},
	}, nil
}

type model interface {
	GetModule() string
	GetVersion() string
	GetType() string
}

func getModelPath(m model) string {
	return buildModelPath(m.GetVersion(), m.GetModule(), m.GetType())
}

// ModelForItem
func ModelForItem(item *api.Item) (registeredModel, error) {
	if data := item.GetData(); data != nil {
		return GetModel(data)
	}
	if id := item.GetId(); id != nil {
		modelPath := getModelPath(id.Model)
		protoName, found := modelPaths[modelPath]
		if found {
			model, _ := registeredModels[protoName]
			return *model, nil
		}
	}

	return registeredModel{}, fmt.Errorf("no model found for item %v", item)
}

func ItemKey(item *api.Item) (string, error) {
	if data := item.GetData(); data != nil {
		return GetKey(data)
	}
	if id := item.GetId(); id != nil {
		modelPath := getModelPath(id.Model)
		protoName, found := modelPaths[modelPath]
		if found {
			model, _ := registeredModels[protoName]
			key := model.KeyPrefix() + id.Name
			return key, nil
		}
	}

	return "", fmt.Errorf("invalid item: %v", item)
}

// RegisteredModels returns all registered modules.
func RegisteredModels() (models []*api.ModelInfo) {
	for _, s := range registeredModels {
		models = append(models, &api.ModelInfo{
			Model: &api.Model{
				Module:  s.Module,
				Type:    s.Type,
				Version: s.Version,
			},
			Info: map[string]string{
				"nameTemplate": s.nameTemplate,
				"protoName":    s.protoName,
				"modelPath":    s.modelPath,
				"keyPrefix":    s.keyPrefix,
			},
		})
	}
	return
}
