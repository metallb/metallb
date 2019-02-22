# NameToIndex

Note: idxvpp2 package will completely replace idxvpp once all plugins are based on
`KVScheduler`.

The NameToIndex mapping is an extension of the NamedMapping mapping. It is
used by VPP Agent plugins that interact with VPP/Linux to map between items
with integer handles and the string-based object identifiers used by northbound
clients of the Agent.

The mappings are primarily used to match VPP dumps with the northbound
configuration. This is essential for the re-configuration and state
re-synchronization after failures.
Furthermore, a mapping registry may be shared between plugins.
For example, `ifplugin` exposes a `sw_if_index->iface_meta` mapping (extended
`NameToIndex`) so that other plugins may reference interfaces from objects
that depend on them, such as bridge domains or IP routes.

**API**

Every plugin is allowed to allocate a new mapping using the function
`NewNameToIndex(logger, title, indexfunction)`, giving in-memory-only
storage capabilities. Specifying `indexFunction` allows to add user-defined
secondary indices.

The `NameToIndexRW` interface supports read and write operations. While the
registry owner is allowed to do both reads and writes, only the read
interface `NameToIndex` is typically exposed to other plugins.

The read-only interface provides item-by-name and item-by-index look-ups using
the `LookupByName` and `LookupByIndex` functions, respectively. Additionally,
a client can use the `WatchItems` function to watch for changes in the registry
related to items with integer handles. The registry owner can change the mapping
content using the `Put/Delete/Update` functions from the underlying NamedMapping.

**KVScheduler-owned mapping**

Plugins configuring VPP items via `KVScheduler` (`ligato/cn-infra/kvscheduler`),
are able to let the scheduler to keep the mapping of item metadata up-to-date.
`WithMetadata()` function of `KVDescriptor` is used to enable/disable
the scheduler-managed mapping for item metadata. Normally, the scheduler uses
the basic `NamedMapping` to keep the association between item name and item
metadata. Descriptor, however, may provide a mapping factory, building mapping
with customized secondary indexes - like `NameToIndex` or its extensions.
The mapping is then available for reading to everyone via scheduler's method
`GetMetadataMap(descriptor)`. For mappings customized using the factory,
the returned `NamedMapping` can be then further casted to interface exposing
the extra look-ups, but keeping the access read-only.

*Example*

Here are some simplified code snippets from `ifplugin` showing how descriptor
can define mapping factory for the scheduler, and how the plugin then propagates
a read-only access to the mapping, including the extra secondary indexes:

```
// ifaceidx extends NameToIndex with IP lookups (for full code see plugins/vpp/ifplugin/ifaceidx2):

type IfaceMetadataIndex interface {
	LookupByName(name string) (metadata *IfaceMetadata, exists bool)
	LookupBySwIfIndex(swIfIndex uint32) (name string, metadata *IfaceMetadata, exists bool)
	LookupByIP(ip string) []string /* name */
	WatchInterfaces(subscriber string, channel chan<- IfaceMetadataDto)
}

type IfaceMetadata struct {
	SwIfIndex   uint32
	IpAddresses []string
}

// In descriptor:

func (intfd *IntfDescriptorImpl) WithMetadata() (withMeta bool, customMapFactory kvscheduler.MetadataMapFactory) {
	return true, func() idxmap.NamedMappingRW {
		return ifaceidx.NewIfaceIndex(logrus.DefaultLogger(), "interface-index")
		}
}

// In ifplugin API:

type IfPlugin struct {
	Deps

	intfIndex ifaceidx.IfaceMetadataIndex
}

func (p *IfPlugin) Init() error {
	descriptor := adapter.NewIntfDescriptor(&descriptor.IntfDescriptorImpl{})
	p.Deps.Scheduler.RegisterKVDescriptor(descriptor)

	var withIndex bool
	metadataMap := p.Deps.Scheduler.GetMetadataMap(descriptor.GetName())
	p.intfIndex, withIndex = metadataMap.(ifaceidx.IfaceMetadataIndex)
	if !withIndex {
		return errors.New("missing index with interface metadata")
	}
	return nil
}

func (p *IfPlugin) GetInterfaceIndex() ifaceidx.IfaceMetadataIndex {
	return p.intfIndex
}
```
