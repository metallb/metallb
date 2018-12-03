package main

import (
	"log"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
		"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/db/keyval/redis"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/utils/safeclose"
)

// PluginName represents name of plugin.
const PluginName = "redis-example"

// Main allows running Example Plugin as a statically linked binary with Agent Core Plugins. Close channel and plugins
// required for the example are initialized. Agent is instantiated with generic plugin (Status check, and Log)
// and example plugin which demonstrates use of Redis flavor.
func main() {
	// Prepare Redis data sync plugin as an plugin dependency
	redisDataSync := kvdbsync.NewPlugin(kvdbsync.UseDeps(func(deps *kvdbsync.Deps) {
		deps.KvPlugin = &etcd.DefaultPlugin
	}))

	// Init example plugin dependencies
	ep := &ExamplePlugin{
		Deps: Deps{
			Log:     logging.ForPlugin(PluginName),
			Watcher: redisDataSync,
			DB:      &redis.DefaultPlugin,
		},
		exampleFinished: make(chan struct{}),
	}

	// Start Agent with example plugin including dependencies
	a := agent.NewAgent(
		agent.AllPlugins(ep),
		agent.QuitOnClose(ep.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExamplePlugin to depict the use of Redis flavor
type ExamplePlugin struct {
	Deps // plugin dependencies are injected

	exampleFinished chan struct{}
}

// Deps is a helper struct which is grouping all dependencies injected to the plugin
type Deps struct {
	Log     logging.PluginLogger
	Watcher datasync.KeyValProtoWatcher
	DB      keyval.KvProtoPlugin
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init is meant for registering the watcher
func (plugin *ExamplePlugin) Init() (err error) {
	//TODO plugin.Watcher.Watch()

	return nil
}

// AfterInit is meant to use DB if needed
func (plugin *ExamplePlugin) AfterInit() (err error) {
	db := plugin.DB.NewBroker(keyval.Root)
	db.ListKeys(keyval.Root)

	return nil
}

// Close is called by Agent Core when the Agent is shutting down. It is supposed to clean up resources that were
// allocated by the plugin during its lifetime
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.exampleFinished)
}
