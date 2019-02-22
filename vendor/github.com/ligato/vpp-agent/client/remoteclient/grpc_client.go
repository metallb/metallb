package remoteclient

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/proto"

	api "github.com/ligato/vpp-agent/api/genericmanager"
	"github.com/ligato/vpp-agent/client"
	"github.com/ligato/vpp-agent/pkg/models"
	"github.com/ligato/vpp-agent/pkg/util"
)

type grpcClient struct {
	remote api.GenericManagerClient
}

// NewClientGRPC returns new instance that uses given service client for requests.
func NewClientGRPC(client api.GenericManagerClient) client.ConfigClient {
	return &grpcClient{client}
}

func (c *grpcClient) KnownModels() ([]api.ModelInfo, error) {
	ctx := context.Background()

	resp, err := c.remote.Capabilities(ctx, &api.CapabilitiesRequest{})
	if err != nil {
		return nil, err
	}

	var modules []api.ModelInfo
	for _, info := range resp.KnownModels {
		modules = append(modules, *info)
	}

	return modules, nil
}

func (c *grpcClient) ChangeRequest() client.ChangeRequest {
	return &setConfigRequest{
		client: c.remote,
		req:    &api.SetConfigRequest{},
	}
}

func (c *grpcClient) ResyncConfig(items ...proto.Message) error {
	req := &api.SetConfigRequest{
		OverwriteAll: true,
	}

	for _, protoModel := range items {
		item, err := models.MarshalItem(protoModel)
		if err != nil {
			return err
		}
		req.Updates = append(req.Updates, &api.UpdateItem{
			Item: item,
		})
	}

	_, err := c.remote.SetConfig(context.Background(), req)
	return err
}

func (c *grpcClient) GetConfig(dsts ...interface{}) error {
	ctx := context.Background()

	resp, err := c.remote.GetConfig(ctx, &api.GetConfigRequest{})
	if err != nil {
		return err
	}

	fmt.Printf("GetConfig: %+v\n", resp)

	protos := map[string]proto.Message{}
	for _, item := range resp.Items {
		val, err := models.UnmarshalItem(item.Item)
		if err != nil {
			return err
		}
		var key string
		if data := item.Item.GetData(); data != nil {
			key, err = models.GetKey(val)
		} else {
			// protos[item.Item.Key] = val
			key, err = models.ItemKey(item.Item)
		}
		if err != nil {
			return err
		}
		protos[key] = val
	}

	util.PlaceProtos(protos, dsts...)

	return nil
}

type setConfigRequest struct {
	client api.GenericManagerClient
	req    *api.SetConfigRequest
	err    error
}

func (r *setConfigRequest) Update(items ...proto.Message) client.ChangeRequest {
	if r.err != nil {
		return r
	}
	for _, protoModel := range items {
		item, err := models.MarshalItem(protoModel)
		if err != nil {
			r.err = err
			return r
		}
		r.req.Updates = append(r.req.Updates, &api.UpdateItem{
			Item: item,
		})
	}
	return r
}

func (r *setConfigRequest) Delete(items ...proto.Message) client.ChangeRequest {
	if r.err != nil {
		return r
	}
	for _, protoModel := range items {
		item, err := models.MarshalItem(protoModel)
		if err != nil {
			if err != nil {
				r.err = err
				return r
			}
		}
		r.req.Updates = append(r.req.Updates, &api.UpdateItem{
			/*Item: &api.Item{
				Key: item.Key,
			},*/
			Item: item,
		})
	}
	return r
}

func (r *setConfigRequest) Send(ctx context.Context) (err error) {
	if r.err != nil {
		return r.err
	}
	_, err = r.client.SetConfig(ctx, r.req)
	return err
}
