# Plugin Lifecycle

Each plugin must implement the Init() and Close() methods (see
[infra.go][1]. A plugin may optionally implement the AfterInit()
method. These methods are called sequentially at startup
by [agent.go][2].

There are following rules for what to put in the methods:
## Init()
* Initialize maps & channels to avoid nil pointers later.
* Process configs (see the [Config Guidelins][3]
* Propagate errors. If an error occurs, the Agent stops since it is not
  properly initialized and calls Close() methods.
* Start watching  GO channels (but not subscribed yet) in a go routine.
* Initialize the GO lang Context & Cancel Function so that go routines
  can be stopped gracefully.

## AfterInit()
* Connect clients & start servers here (see the
  [System Integration Guidelines][4]
* Propagate errors. Agent will stop because it is not properly
  initialized and calls Close() methods.
* Subscribe for watching data (see the go channel in the example below).

## Close()
* Cancel the go routines by calling GO lang (Context) Cancel function.
* Disconnect clients & stop servers, release resources. Try achieving
  that using the [safeclose](../../utils/safeclose) package.
* Propagate errors. Agent will log those errors.

## Example
```go
package example
import (
    "errors"
    "context"
    "io"
    "github.com/ligato/cn-infra/datasync"
    "github.com/ligato/cn-infra/logging"
    "github.com/ligato/cn-infra/utils/safeclose"
)

type PluginXY struct {
    Dep
    resource    io.Closer
    dataChange  chan datasync.ChangeEvent
    dataResync  chan datasync.ResyncEvent
    data        map[string]interface{}
    cancel      context.CancelFunc
}

type Dep struct {
    Watcher     datasync.KeyValProtoWatcher // Inject
    Logger      logging.Logger              // Inject
    ParentCtx   context.Context             // inject optionally
}

func (plugin * PluginXY) Init() (err error) {
    //initialize the resource
    if plugin.resource, err = connectResource(); err != nil {
        return err//propagate resource
    }


    // initialize maps (to avoid segmentation fault)
    plugin.data = make(map[string]interface{})

    // initialize channels & start go routines
    plugin.dataChange = make(chan datasync.ChangeEvent, 100)
    plugin.dataResync = make(chan datasync.ResyncEvent, 100)

    // initiate context & cancel function (to stop go routine)
    var ctx context.Context
    if plugin.ParentCtx == nil {
        ctx, plugin.cancel = context.WithCancel(context.Background())
    } else {
        ctx, plugin.cancel = context.WithCancel(plugin.ParentCtx)
    }

    go func() {
        for {
            select {
            case dataChangeEvent := <-plugin.dataChange:
                plugin.Logger.Debug(dataChangeEvent)
            case dataResyncEvent := <-plugin.dataResync:
                plugin.Logger.Debug(dataResyncEvent)
            case <-ctx.Done():
                // stop watching for notifications
                return
            }
        }
    }()

    return nil
}

func connectResource() (resource io.Closer, err error) {
    // do something relevant here...
    return nil, errors.New("Not implemented")
}

func (plugin * PluginXY) AfterInit() error {
    // subscribe plugin.channel for watching data (to really receive the data)
    plugin.Watcher.Watch("watchingXY", plugin.dataChange, plugin.dataResync, "keysXY")

    return nil
}

func (plugin * PluginXY) Close() error {
    // cancel watching the channels
    plugin.cancel()

    // close all resources / channels
    _, err := safeclose.CloseAll(plugin.dataChange, plugin.dataResync, plugin.resource)
    return err
}
```
[1]: ../../infra/infra.go
[2]: ../../agent/agent.go
[3]: CONFIG.md
[4]: SYSTEM_INTEGRATION.md
