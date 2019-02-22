//  Copyright (c) 2019 Cisco and/or its affiliates.
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

package orchestrator

import (
	"github.com/gogo/protobuf/proto"
	"github.com/gogo/status"
	"github.com/ligato/cn-infra/logging"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	api "github.com/ligato/vpp-agent/api/genericmanager"
	"github.com/ligato/vpp-agent/pkg/models"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
)

type genericManagerSvc struct {
	log  logging.Logger
	orch *Plugin
}

func (s *genericManagerSvc) Capabilities(ctx context.Context, req *api.CapabilitiesRequest) (*api.CapabilitiesResponse, error) {
	resp := &api.CapabilitiesResponse{
		KnownModels: models.RegisteredModels(),
	}
	return resp, nil
}

func (s *genericManagerSvc) SetConfig(ctx context.Context, req *api.SetConfigRequest) (*api.SetConfigResponse, error) {
	s.log.Debug("------------------------------")
	s.log.Debugf("=> Configurator.SetConfig: %d items", len(req.Updates))
	s.log.Debug("------------------------------")
	for _, item := range req.Updates {
		s.log.Debugf(" - %v", item)
	}
	s.log.Debug("------------------------------")

	if req.OverwriteAll {
		ctx = kvs.WithResync(ctx, kvs.FullResync, true)
	}

	var ops = make(map[string]api.UpdateResult_Operation)
	var kvPairs []KeyValuePair

	for _, update := range req.Updates {
		item := update.Item
		/*if item == nil {
			return nil, status.Error(codes.InvalidArgument, "change item is nil")
		}*/
		var (
			key string
			val proto.Message
		)

		var err error
		if item.Data != nil {
			val, err = models.UnmarshalItem(item)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			key, err = models.GetKey(val)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			ops[key] = api.UpdateResult_UPDATE
		} else if item.Id != nil {
			model, err := models.ModelForItem(item)
			if err != nil {
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			key := model.KeyPrefix() + item.Id.Name
			ops[key] = api.UpdateResult_DELETE
		} else {
			return nil, status.Error(codes.InvalidArgument, "ProtoItem has no key or val defined.")
		}
		kvPairs = append(kvPairs, KeyValuePair{
			Key:   key,
			Value: val,
		})
	}

	kvErrs, err := s.orch.PushData(ctx, kvPairs)
	if err != nil {
		st := status.New(codes.FailedPrecondition, err.Error())
		return nil, st.Err()
	}

	results := []*api.UpdateResult{}
	for _, kvErr := range kvErrs {
		results = append(results, &api.UpdateResult{
			Key: kvErr.Key,
			Status: &api.ItemStatus{
				//State:   api.ItemStatus_FAILURE,
				Message: kvErr.Error.Error(),
			},
			Op: ops[kvErr.Key],
		})
	}

	/*
		// commit the transaction
		if err := txn.Commit(); err != nil {
			st := status.New(codes.FailedPrecondition, err.Error())
			return nil, st.Err()
			// TODO: use the WithDetails to return extra info to clients.
			//ds, err := st.WithDetails(&rpc.DebugInfo{Detail: "Local transaction failed!"})
			//if err != nil {
			//	return nil, st.Err()
			//}
			//return nil, ds.Err()
		}
	*/

	return &api.SetConfigResponse{Results: results}, nil
}

func (s *genericManagerSvc) GetConfig(context.Context, *api.GetConfigRequest) (*api.GetConfigResponse, error) {
	var items []*api.GetConfigResponse_ConfigItem

	for _, data := range s.orch.ListData() {
		item, err := models.MarshalItem(data)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		items = append(items, &api.GetConfigResponse_ConfigItem{
			Item: item,
		})
	}

	return &api.GetConfigResponse{Items: items}, nil
}

func (s *genericManagerSvc) DumpState(context.Context, *api.DumpStateRequest) (*api.DumpStateResponse, error) {
	panic("implement me")
}

func (s *genericManagerSvc) Subscribe(req *api.SubscribeRequest, server api.GenericManager_SubscribeServer) error {
	panic("implement me")
}
