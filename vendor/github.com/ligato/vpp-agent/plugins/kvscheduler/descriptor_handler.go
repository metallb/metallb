package kvscheduler

import (
	"github.com/gogo/protobuf/proto"
	kvs "github.com/ligato/vpp-agent/plugins/kvscheduler/api"
)

// descriptorHandler handles access to descriptor methods (callbacks).
// For callback not provided, a default return value is returned.
type descriptorHandler struct {
	descriptor *kvs.KVDescriptor
}

// keyLabel by default returns the key itself.
func (h *descriptorHandler) keyLabel(key string) string {
	if h.descriptor == nil || h.descriptor.KeyLabel == nil {
		return key
	}
	return h.descriptor.KeyLabel(key)
}

// equivalentValues by default uses proto.Equal().
func (h *descriptorHandler) equivalentValues(key string, oldValue, newValue proto.Message) bool {
	if h.descriptor == nil || h.descriptor.ValueComparator == nil {
		return proto.Equal(oldValue, newValue)
	}
	return h.descriptor.ValueComparator(key, oldValue, newValue)
}

// validate return nil if Validate is not provided (optional method).
func (h *descriptorHandler) validate(key string, value proto.Message) error {
	if h.descriptor == nil || h.descriptor.Validate == nil {
		return nil
	}
	return h.descriptor.Validate(key, value)
}

// add returns ErrUnimplementedAdd is Add is not provided.
func (h *descriptorHandler) add(key string, value proto.Message) (metadata kvs.Metadata, err error) {
	if h.descriptor == nil {
		return
	}
	if h.descriptor.Add == nil {
		return nil, kvs.ErrUnimplementedAdd
	}
	return h.descriptor.Add(key, value)
}

// modify returns ErrUnimplementedModify if Modify is not provided.
func (h *descriptorHandler) modify(key string, oldValue, newValue proto.Message, oldMetadata kvs.Metadata) (newMetadata kvs.Metadata, err error) {
	if h.descriptor == nil {
		return oldMetadata, nil
	}
	if h.descriptor.Modify == nil {
		return oldMetadata, kvs.ErrUnimplementedModify
	}
	return h.descriptor.Modify(key, oldValue, newValue, oldMetadata)
}

// modifyWithRecreate by default assumes any change can be applied using Modify without
// re-creation.
func (h *descriptorHandler) modifyWithRecreate(key string, oldValue, newValue proto.Message, metadata kvs.Metadata) bool {
	if h.descriptor == nil || h.descriptor.ModifyWithRecreate == nil {
		return false
	}
	return h.descriptor.ModifyWithRecreate(key, oldValue, newValue, metadata)
}

// delete returns ErrUnimplementedDelete if Delete is not provided.
func (h *descriptorHandler) delete(key string, value proto.Message, metadata kvs.Metadata) error {
	if h.descriptor == nil {
		return nil
	}
	if h.descriptor.Delete == nil {
		return kvs.ErrUnimplementedDelete
	}
	return h.descriptor.Delete(key, value, metadata)
}

// isRetriableFailure first checks for errors returned by the handler itself.
// If descriptor does not define IsRetriableFailure, it is assumed any failure
// can be potentially fixed by retry.
func (h *descriptorHandler) isRetriableFailure(err error) bool {
	// first check for errors returned by the handler itself
	handlerErrs := []error{kvs.ErrUnimplementedAdd, kvs.ErrUnimplementedModify, kvs.ErrUnimplementedDelete}
	for _, handlerError := range handlerErrs {
		if err == handlerError {
			return false
		}
	}
	if h.descriptor == nil || h.descriptor.IsRetriableFailure == nil {
		return true
	}
	return h.descriptor.IsRetriableFailure(err)
}

// dependencies returns empty list if descriptor does not define any.
func (h *descriptorHandler) dependencies(key string, value proto.Message) (deps []kvs.Dependency) {
	if h.descriptor == nil || h.descriptor.Dependencies == nil {
		return
	}
	return h.descriptor.Dependencies(key, value)
}

// derivedValues returns empty list if descriptor does not define any.
func (h *descriptorHandler) derivedValues(key string, value proto.Message) (derives []kvs.KeyValuePair) {
	if h.descriptor == nil || h.descriptor.DerivedValues == nil {
		return
	}
	return h.descriptor.DerivedValues(key, value)
}

// dump returns <ableToDump> as false if descriptor does not implement Dump.
func (h *descriptorHandler) dump(correlate []kvs.KVWithMetadata) (dump []kvs.KVWithMetadata, ableToDump bool, err error) {
	if h.descriptor == nil || h.descriptor.Dump == nil {
		return dump, false, nil
	}
	dump, err = h.descriptor.Dump(correlate)
	return dump, true, err
}
