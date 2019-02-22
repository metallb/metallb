package main

import (
	"log"
	"strings"
	"time"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/datasync"
	"github.com/ligato/cn-infra/datasync/kvdbsync"
	"github.com/ligato/cn-infra/datasync/resync"
	"github.com/ligato/cn-infra/db/keyval/etcd"
	"github.com/ligato/cn-infra/examples/model"
	"github.com/ligato/cn-infra/logging"
	"github.com/ligato/cn-infra/servicelabel"
	"github.com/ligato/cn-infra/utils/safeclose"
	"golang.org/x/net/context"
)

// *************************************************************************
// This example demonstrates the usage of datasync API with etcd
// as the data store.
// ExamplePlugin spawns a data publisher and a data consumer (watcher)
// as two separate go routines.
// The publisher executes two operations on the same key: CREATE + UPDATE.
// The consumer is notified with each change and reports the events into
// the log.
// ************************************************************************/

// PluginName represents name of plugin.
const PluginName = "datasync-example"

func main() {
	// Prepare ETCD data sync plugin as an plugin dependency
	etcdDataSync := kvdbsync.NewPlugin(kvdbsync.UseDeps(func(deps *kvdbsync.Deps) {
		deps.KvPlugin = &etcd.DefaultPlugin
	}))
	// Init example plugin dependencies
	p := &ExamplePlugin{
		Deps: Deps{
			Log:          logging.ForPlugin(PluginName),
			ServiceLabel: &servicelabel.DefaultPlugin,
			Publisher:    etcdDataSync,
			Watcher:      etcdDataSync,
		},
		exampleFinished: make(chan struct{}),
	}
	// Start Agent with example plugin including dependencies
	a := agent.NewAgent(
		agent.AllPlugins(p),
		agent.QuitOnClose(p.exampleFinished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// ExamplePlugin demonstrates the usage of datasync API.
type ExamplePlugin struct {
	Deps

	changeChannel chan datasync.ChangeEvent // Channel used by the watcher for change events.
	resyncChannel chan datasync.ResyncEvent // Channel used by the watcher for resync events.
	context       context.Context           // Used to cancel watching.
	cancel        context.CancelFunc
	watchDataReg  datasync.WatchRegistration // To subscribe on data change/resync events.
	// Fields below are used to properly finish the example.
	eventCounter  uint8
	publisherDone bool

	exampleFinished chan struct{}
}

// Deps lists dependencies of ExamplePlugin.
type Deps struct {
	Log          logging.PluginLogger
	ServiceLabel servicelabel.ReaderAPI
	Publisher    datasync.KeyProtoValWriter  // injected - To write ETCD data
	Watcher      datasync.KeyValProtoWatcher // injected - To watch ETCD data
}

// String returns plugin name
func (plugin *ExamplePlugin) String() string {
	return PluginName
}

// Init starts the consumer.
func (plugin *ExamplePlugin) Init() error {
	// Initialize plugin fields.
	plugin.resyncChannel = make(chan datasync.ResyncEvent)
	plugin.changeChannel = make(chan datasync.ChangeEvent)
	plugin.context, plugin.cancel = context.WithCancel(context.Background())

	// Start the consumer (ETCD watcher).
	go plugin.consumer()

	// Subscribe watcher to be able to watch on data changes and resync events.
	err := plugin.subscribeWatcher()
	if err != nil {
		return err
	}

	plugin.Log.Info("Initialization of the custom plugin for the datasync example is completed")

	return nil
}

// AfterInit starts the publisher and prepares for the shutdown.
func (plugin *ExamplePlugin) AfterInit() error {
	resync.DefaultPlugin.DoResync()

	go plugin.etcdPublisher()

	go plugin.closeExample()

	return nil
}

// Close shutdowns both the publisher and the consumer.
// Channels used to propagate data resync and data change events are closed
// as well.
func (plugin *ExamplePlugin) Close() error {
	return safeclose.Close(plugin.resyncChannel, plugin.changeChannel)
}

// subscribeWatcher subscribes for data change and data resync events.
// Events are delivered to the consumer via the selected channels.
// ETCD watcher adapter is used to perform the registration behind the scenes.
func (plugin *ExamplePlugin) subscribeWatcher() (err error) {
	prefix := etcdKeyPrefix(plugin.ServiceLabel.GetAgentLabel())
	plugin.Log.Infof("Prefix: %v", prefix)
	plugin.watchDataReg, err = plugin.Watcher.
		Watch("ExamplePlugin", plugin.changeChannel, plugin.resyncChannel, prefix)
	if err != nil {
		return err
	}

	plugin.Log.Info("KeyValProtoWatcher subscribed")

	return nil
}

// etcdPublisher creates a simple data, then demonstrates CREATE and UPDATE
// operations with ETCD.
func (plugin *ExamplePlugin) etcdPublisher() {
	// Wait for the consumer to initialize
	time.Sleep(1 * time.Second)

	plugin.Log.Print("KeyValPublisher started")

	// Convert data into the proto format.
	exampleData := plugin.buildData("string1", 0, true)

	// PUT: demonstrate how to use the Data Broker Put() API to store
	// a simple data structure into ETCD.
	label := etcdKeyPrefixLabel(plugin.ServiceLabel.GetAgentLabel(), "index")
	plugin.Log.Infof("Write data to %v", label)
	plugin.Publisher.Put(label, exampleData)

	// Prepare different set of data.
	plugin.Log.Infof("Update data at %v", label)
	exampleData = plugin.buildData("string2", 1, false)

	// UPDATE: demonstrate how use the Data Broker Put() API to change
	// an already stored data in ETCD.
	plugin.Publisher.Put(label, exampleData)

	// Prepare another different set of data.
	plugin.Log.Infof("Update data at %v", label)
	exampleData = plugin.buildData("string3", 2, false)

	// UPDATE: only to demonstrate Unregister functionality
	plugin.Publisher.Put(label, exampleData)

	// Wait for the consumer (change should not be passed to listener)
	time.Sleep(2 * time.Second)

	plugin.publisherDone = true
}

// consumer (watcher) is subscribed to watch on data store changes.
// Changes arrive via data change channel, get identified based on the key
// and printed into the log.
func (plugin *ExamplePlugin) consumer() {
	plugin.Log.Print("KeyValProtoWatcher started")
	for {
		select {
		// WATCH: demonstrate how to receive data change events.
		case dataEv := <-plugin.changeChannel:
			plugin.Log.Printf("Received event: %v", dataEv)
			// If event arrives, the key is extracted and used together with
			// the expected prefix to identify item.

			for _, dataChng := range dataEv.GetChanges() {
				key := dataChng.GetKey()
				if strings.HasPrefix(key, etcdKeyPrefix(plugin.ServiceLabel.GetAgentLabel())) {
					var value, previousValue etcdexample.EtcdExample
					// The first return value is diff - boolean flag whether previous value exists or not
					err := dataChng.GetValue(&value)
					if err != nil {
						plugin.Log.Error(err)
					}
					diff, err := dataChng.GetPrevValue(&previousValue)
					if err != nil {
						plugin.Log.Error(err)
					}
					plugin.Log.Infof("Event arrived to etcd eventHandler, key %v, update: %v, change type: %v,",
						dataChng.GetKey(), diff, dataChng.GetChangeType())
					// Increase event counter (expecting two events).
					plugin.eventCounter++

					if plugin.eventCounter == 2 {
						// After creating/updating data, unregister key
						plugin.Log.Infof("Unregister key %v", etcdKeyPrefix(plugin.ServiceLabel.GetAgentLabel()))
						plugin.watchDataReg.Unregister(etcdKeyPrefix(plugin.ServiceLabel.GetAgentLabel()))
					}
				}
			}

			// Here you would test for other event types with one if statement
			// for each key prefix:
			//
			// if strings.HasPrefix(key, etcd prefix) { ... }

			// Here you would also watch for resync events
			// (not published in this example):
			//
			// case resyncEvent := <-plugin.ResyncEvent:
			//   ...
		case rs := <-plugin.resyncChannel:
			// Resync event notification
			plugin.Log.Infof("Resync event %v called", rs)
			rs.Done(nil)
		case <-plugin.context.Done():
			plugin.Log.Debugf("Stop watching events")
			return
		}
	}
}

func (plugin *ExamplePlugin) closeExample() {
	for {
		// Two events are expected for successful example completion.
		if plugin.publisherDone {
			if plugin.eventCounter != 2 {
				plugin.Log.Error("etcd/datasync example failed", plugin.eventCounter)
			}
			// Close the watcher
			plugin.cancel()
			plugin.Log.Infof("etcd/datasync example finished, sending shutdown ...")
			// Close the example
			close(plugin.exampleFinished)
			break
		}
	}
}

// Create simple ETCD data structure with provided data values.
func (plugin *ExamplePlugin) buildData(stringVal string, uint32Val uint32, boolVal bool) *etcdexample.EtcdExample {
	return &etcdexample.EtcdExample{
		StringVal: stringVal,
		Uint32Val: uint32Val,
		BoolVal:   boolVal,
	}
}

// The ETCD key prefix used for this example
func etcdKeyPrefix(agentLabel string) string {
	return "/vnf-agent/" + agentLabel + "/api/v1/example/db/simple/"
}

// The ETCD key (the key prefix + label)
func etcdKeyPrefixLabel(agentLabel string, index string) string {
	return etcdKeyPrefix(agentLabel) + index
}
