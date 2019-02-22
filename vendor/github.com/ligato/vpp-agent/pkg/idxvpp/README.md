# NameToIdx

The NameToIdx mapping is an extension of the NamedMapping mapping. It is 
used by VPP Agent plugins that interact with VPP to map between VPP 
interface handles (the `sw_if_index` entities) and the string-based 
object identifiers used by northbound clients of the Agent.

The mappings are used to implement the re-configuration and state 
re-synchronization after failures. Furthermore, a mapping registry may
be shared between plugins. For example, `ifplugin` exposes the 
`sw_if_index->name` mapping so that other plugins may reference interfaces
from objects that depend on them, such as bridge domains or IP routes.

**API**

*Mapping*

Every plugin is allowed to allocate a new mapping using the function 
`NewNameToIdxRW(logger, owner, title, indexfunction)`, giving in-memory-only
storage capabilities. Specifying an indexFunction allows to query mappings
by secondary indices computed from metadata.
 
The `NameToIdxRW` interface supports read and write operations. While the 
registry owner is allowed to do both reads and writes, only the read 
interface `NameToIdx` is typically exposed to the other plugins. See for 
example the `sw_if_index->name` mapping defined in `ifplugin`. Its read-only
interface supports index-by-name and name-by-index look-ups using the 
`LookupIdx` and `LookupName` functions. Additionally, a client can use the 
`Watch` function to watch for changes in the registry. The registry owner 
can  register a new mapping using the `RegisterName` function and remove 
an existing mapping using the `UnregisterName` function.

**Example**

Here is a simplified code snippet from `ifplugin` showing how to use the 
`sw_if_index->name` mapping:

```
// Plugin allocates new registries by its name and automatically becomes
// their owner.
const PluginID pluginapi.PluginName = "ifplugin"

// InterfaceMeta defines the attributes of metadata as used by the 
// interface plugin.
type InterfaceMeta struct {
	InterfaceType intf.InterfaceType
}

// Init initializes the interface plugin
func (plugin *InterfaceConfigurator) Init() {
    // Allocate registry for sw_if_index to name mappings.
    plugin.swIfIndexes, err = idxmap.NewNameToIdx(logger, PluginID, "sw_if_indexes", nil)
    if err != nil {
        // handle error
    }
    
    // Continue with the initialization...
}

// ConfigureInterface configures a new VPP or Linux interface.
func (plugin *InterfaceConfigurator) ConfigureInterface(iface *intf.Interfaces_Interface) {
    // Create the interface ...
    // ifIdx := ...
    
    
    // Once a new interface is created in VPP/Linux, add new mapping into the registry
    // if it doesn't exist yet
    _, _, found := plugin.SwIfIndexes.LookupName(ifIdx)
    if !found {
        plugin.SwIfIndexes.RegisterName(iface.Name, ifIdx, &InterfaceMeta{iface.Type})
    }
}

// DeleteInterface removes an existing VPP or Linux interface.
func (plugin *InterfaceConfigurator) DeleteInterface(iface *intf.Interfaces_Interface) {
    // Delete the interface ...
    
    // When the interface gets deleted from VPP/Linux, the mapping must be removed as well.
    plugin.SwIfIndexes.UnregisterName(iface.Name)
}
```
