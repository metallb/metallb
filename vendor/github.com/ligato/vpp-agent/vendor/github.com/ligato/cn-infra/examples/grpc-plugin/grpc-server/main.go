package main

import (
	"errors"
	"log"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/logging/logrus"
	"github.com/ligato/cn-infra/rpc/grpc"
	"github.com/ligato/cn-infra/rpc/rest"
	"golang.org/x/net/context"
	"google.golang.org/grpc/examples/helloworld/helloworld"
)

// *************************************************************************
// This file contains GRPC service exposure example. To register service use
// Server.RegisterService(descriptor, service)
// ************************************************************************/

// PluginName represents name of plugin.
const PluginName = "myPlugin"

func main() {
	p := &ExamplePlugin{
		GRPC: grpc.NewPlugin(
			grpc.UseHTTP(&rest.DefaultPlugin),
		),
		Log: logging.ForPlugin(PluginName),
	}

	a := agent.NewAgent(agent.AllPlugins(p))

	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExamplePlugin presents main plugin.
type ExamplePlugin struct {
	Log  logging.PluginLogger
	GRPC grpc.Server
}

// String return name of the plugin.
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init demonstrates the usage of PluginLogger API.
func (plugin *ExamplePlugin) Init() error {
	plugin.Log.Info("Registering greeter")

	helloworld.RegisterGreeterServer(plugin.GRPC.GetServer(), &GreeterService{})

	return nil
}

// Close closes the plugin.
func (plugin *ExamplePlugin) Close() error {
	return nil
}

// GreeterService implements GRPC GreeterServer interface (interface generated from protobuf definition file).
// It is a simple implementation for testing/demo only purposes.
type GreeterService struct{}

// SayHello returns error if request.name was not filled otherwise: "hello " + request.Name
func (*GreeterService) SayHello(ctx context.Context, request *helloworld.HelloRequest) (*helloworld.HelloReply, error) {
	if request.Name == "" {
		return nil, errors.New("not filled name in the request")
	}
	logrus.DefaultLogger().Infof("greeting client: %v", request.Name)

	return &helloworld.HelloReply{Message: "hello " + request.Name}, nil
}
