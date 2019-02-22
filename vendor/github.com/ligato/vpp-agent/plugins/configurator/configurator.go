package configurator

import (
	"github.com/gogo/status"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/vpp-agent/api/models/linux"
	"github.com/ligato/vpp-agent/api/models/vpp"
	"github.com/ligato/vpp-agent/pkg/util"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"

	rpc "github.com/ligato/vpp-agent/api/configurator"
	"github.com/ligato/vpp-agent/pkg/models"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/orchestrator"
)

// configuratorServer implements DataSyncer service.
type configuratorServer struct {
	dumpService
	notifyService

	log      logging.Logger
	dispatch *orchestrator.Plugin
}

func (svc *configuratorServer) Get(context.Context, *rpc.GetRequest) (*rpc.GetResponse, error) {
	config := newConfig()

	util.PlaceProtos(svc.dispatch.ListData(), config.LinuxConfig, config.VppConfig)

	return &rpc.GetResponse{Config: config}, nil
}

// Update adds configuration data present in data request to the VPP/Linux
func (svc *configuratorServer) Update(ctx context.Context, req *rpc.UpdateRequest) (*rpc.UpdateResponse, error) {
	protos := util.ExtractProtos(req.Update.VppConfig, req.Update.LinuxConfig)

	var kvPairs []orchestrator.KeyValuePair
	for _, p := range protos {
		key, err := models.GetKey(p)
		if err != nil {
			svc.log.Debug("models.GetKey failed: %s", err)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		kvPairs = append(kvPairs, orchestrator.KeyValuePair{
			Key:   key,
			Value: p,
		})
	}

	if req.FullResync {
		ctx = kvs.WithResync(ctx, kvs.FullResync, true)
	}

	if _, err := svc.dispatch.PushData(ctx, kvPairs); err != nil {
		st := status.New(codes.FailedPrecondition, err.Error())
		return nil, st.Err()
	}

	return &rpc.UpdateResponse{}, nil
}

// Delete removes configuration data present in data request from the VPP/linux
func (svc *configuratorServer) Delete(ctx context.Context, req *rpc.DeleteRequest) (*rpc.DeleteResponse, error) {
	protos := util.ExtractProtos(req.Delete.VppConfig, req.Delete.LinuxConfig)

	var kvPairs []orchestrator.KeyValuePair
	for _, p := range protos {
		key, err := models.GetKey(p)
		if err != nil {
			svc.log.Debug("models.GetKey failed: %s", err)
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		kvPairs = append(kvPairs, orchestrator.KeyValuePair{
			Key:   key,
			Value: nil,
		})
	}

	if _, err := svc.dispatch.PushData(ctx, kvPairs); err != nil {
		st := status.New(codes.FailedPrecondition, err.Error())
		return nil, st.Err()
	}

	return &rpc.DeleteResponse{}, nil
}

func newConfig() *rpc.Config {
	return &rpc.Config{
		LinuxConfig: &linux.ConfigData{},
		VppConfig:   &vpp.ConfigData{},
	}
}
