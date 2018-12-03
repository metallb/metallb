package main

import (
	"log"

	"github.com/ligato/cn-infra/agent"
	"github.com/ligato/cn-infra/db/keyval"
	"github.com/ligato/cn-infra/db/keyval/bolt"
	"github.com/ligato/cn-infra/logging"
)

const pluginName = "bolt-example"

func main() {
	example := &BoltExample{
		Log:      logging.ForPlugin(pluginName),
		DB:       &bolt.DefaultPlugin,
		finished: make(chan struct{}),
	}

	a := agent.NewAgent(
		agent.AllPlugins(example),
		agent.QuitOnClose(example.finished),
	)
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}

// BoltExample demonstrates the usage of Bolt plugin.
type BoltExample struct {
	Log logging.PluginLogger
	DB  keyval.KvProtoPlugin

	finished chan struct{}
}

// Init demonstrates using Bolt plugin.
func (p *BoltExample) Init() (err error) {
	db := p.DB.NewBroker(keyval.Root)

	// Store some data
	txn := db.NewTxn()
	txn.Put("/agent/config/interface/iface0", nil)
	txn.Put("/agent/config/interface/iface1", nil)
	txn.Commit()

	// List keys
	const listPrefix = "/agent/config/interface/"

	p.Log.Infof("List BoltDB keys: %s", listPrefix)

	keys, err := db.ListKeys(listPrefix)
	if err != nil {
		p.Log.Fatal(err)
	}

	for {
		key, val, all := keys.GetNext()
		if all == true {
			break
		}

		p.Log.Infof("Key: %q Val: %v", key, val)
	}

	return nil
}

// AfterInit closes the example.
func (p *BoltExample) AfterInit() (err error) {
	close(p.finished)
	return nil
}

// Close frees plugin resources.
func (p *BoltExample) Close() error {
	return nil
}

// String returns name of plugin.
func (p *BoltExample) String() string {
	return pluginName
}
