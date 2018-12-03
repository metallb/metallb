# Plugin Dependencies

A plugin may simply depend on a value of a basic data-type parameter,
but more often each dependency is a specific feature expressed through
an interface, e.g.:

```go
	type Deps struct {
	    HTTPport string // a basic data-type parameter
	    Publish  datasync.KeyProtoValWriter // feature described by an interface
	}
```

Plugin dependencies are typically listed in a separate structure labeled
as "Deps". This structure is then embedded into the plugin's top
structure so that the injected dependencies can be accessed directly
as the other plugin (internal) fields.
```go
	package xy
	import (
	    "github.com/ligato/cn-infra/flavors/local"
	    "github.com/ligato/cn-infra/datasync"
	)

	type PluginXY struct {
	    // dependencies
	    Deps

		// other fields (usually private fields) ...
	}

	type Deps struct {
	    local.PluginLogDeps //Plugin Logger & Plugin Name

	    // other dependencies:

	    Watcher datasync.KeyValProtoWatcher
	    // ...
	}

    func (plugin *PluginXY) Init() error {
        //using the dependency (following line is shortcut for plugin.Dep.PluginLogDeps.Log)
        plugin.Log.Info("using injected logger in flavor")

        return nil
    }

```

The combination of exported dependency fields together with the Init()
method makes plugin constructors unnecessary.
Therefore, plugins can be listed in the flavors directly as instances
of structures, without the use of pointers.

Whenever possible, try to make the dependencies optional.
Inside the Init() method, plugin should first check for each dependency
if it was successfully injected and use only the resolved ones.
If a mandatory dependency was not injected, the plugin should return
self-explanatory error and not panic.
