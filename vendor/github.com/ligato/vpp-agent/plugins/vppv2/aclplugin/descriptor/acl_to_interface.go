package descriptor

import (
	"github.com/gogo/protobuf/proto"
	"github.com/ligato/cn-infra/logging"
	"github.com/pkg/errors"

	acl "github.com/ligato/vpp-agent/api/models/vpp/acl"
	interfaces "github.com/ligato/vpp-agent/api/models/vpp/interfaces"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/aclidx"
	"github.com/ligato/vpp-agent/plugins/vppv2/aclplugin/vppcalls"
)

const (
	// ACLToInterfaceDescriptorName is name for descriptor
	ACLToInterfaceDescriptorName = "vpp-acl-to-interface"

	// dependency labels
	interfaceDep = "interface-exists"
)

// ACLToInterfaceDescriptor represents assignment of ACL to interface.
type ACLToInterfaceDescriptor struct {
	log        logging.Logger
	aclHandler vppcalls.ACLVppAPI
	aclIndex   aclidx.ACLMetadataIndex
}

// NewACLToInterfaceDescriptor returns new ACLInterface descriptor
func NewACLToInterfaceDescriptor(aclIndex aclidx.ACLMetadataIndex, aclHandler vppcalls.ACLVppAPI,
	log logging.PluginLogger) *ACLToInterfaceDescriptor {
	return &ACLToInterfaceDescriptor{
		log:        log,
		aclIndex:   aclIndex,
		aclHandler: aclHandler,
	}
}

// GetDescriptor returns descriptor suitable for registration with the KVScheduler.
func (d *ACLToInterfaceDescriptor) GetDescriptor() *kvs.KVDescriptor {
	return &kvs.KVDescriptor{
		Name:         ACLToInterfaceDescriptorName,
		KeySelector:  d.IsACLInterfaceKey,
		Add:          d.Add,
		Delete:       d.Delete,
		Dependencies: d.Dependencies,
	}
}

// IsACLInterfaceKey returns true if the key is identifying ACL interface (derived value)
func (d *ACLToInterfaceDescriptor) IsACLInterfaceKey(key string) bool {
	_, _, _, isACLToInterfaceKey := acl.ParseACLToInterfaceKey(key)
	return isACLToInterfaceKey
}

// Add binds interface to ACL.
func (d *ACLToInterfaceDescriptor) Add(key string, emptyVal proto.Message) (metadata kvs.Metadata, err error) {
	aclName, ifName, flow, _ := acl.ParseACLToInterfaceKey(key)

	aclMeta, found := d.aclIndex.LookupByName(aclName)
	if !found {
		err = errors.Errorf("failed to obtain metadata for ACL %s", aclName)
		d.log.Error(err)
		return nil, err
	}

	if aclMeta.L2 {
		// MACIP ACL (L2)
		if err := d.aclHandler.AddMACIPACLToInterface(aclMeta.Index, ifName); err != nil {
			d.log.Error(err)
			return nil, err
		}
	} else {
		// ACL (L3/L4)
		if flow == acl.IngressFlow {
			if err := d.aclHandler.AddACLToInterfaceAsIngress(aclMeta.Index, ifName); err != nil {
				d.log.Error(err)
				return nil, err
			}
		} else if flow == acl.EgressFlow {
			if err := d.aclHandler.AddACLToInterfaceAsEgress(aclMeta.Index, ifName); err != nil {
				d.log.Error(err)
				return nil, err
			}
		}
	}

	return nil, nil
}

// Delete unbinds interface from ACL.
func (d *ACLToInterfaceDescriptor) Delete(key string, emptyVal proto.Message, metadata kvs.Metadata) error {
	aclName, ifName, flow, _ := acl.ParseACLToInterfaceKey(key)

	aclMeta, found := d.aclIndex.LookupByName(aclName)
	if !found {
		err := errors.Errorf("failed to obtain metadata for ACL %s", aclName)
		d.log.Error(err)
		return err
	}

	if aclMeta.L2 {
		// MACIP ACL (L2)
		if err := d.aclHandler.DeleteMACIPACLFromInterface(aclMeta.Index, ifName); err != nil {
			d.log.Error(err)
			return err
		}
	} else {
		// ACL (L3/L4)
		if flow == acl.IngressFlow {
			if err := d.aclHandler.DeleteACLFromInterfaceAsIngress(aclMeta.Index, ifName); err != nil {
				d.log.Error(err)
				return err
			}
		} else if flow == acl.EgressFlow {
			if err := d.aclHandler.DeleteACLFromInterfaceAsEgress(aclMeta.Index, ifName); err != nil {
				d.log.Error(err)
				return err
			}
		}
	}

	return nil
}

// Dependencies lists the interface as the only dependency for the binding.
func (d *ACLToInterfaceDescriptor) Dependencies(key string, emptyVal proto.Message) []kvs.Dependency {
	_, ifName, _, _ := acl.ParseACLToInterfaceKey(key)
	return []kvs.Dependency{
		{
			Label: interfaceDep,
			Key:   interfaces.InterfaceKey(ifName),
		},
	}
}
