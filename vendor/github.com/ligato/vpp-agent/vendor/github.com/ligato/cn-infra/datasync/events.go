package datasync

import (
	"github.com/gogo/protobuf/proto"
)

// Op represents datasync operations.
type Op string

const (
	// InvalidOp is default value
	// representing invalid operation.
	InvalidOp Op = ""
	// Put can represent Create or Update operation.
	Put Op = "Put"
	// Delete represents Delete operation
	Delete Op = "Delete"
)

// CallbackResult can be used by an event receiver to indicate to the event
// producer whether an operation was successful (error is nil) or unsuccessful
// (error is not nil).
type CallbackResult interface {
	// Done allows plugins that are processing data change/resync to send
	// feedback. If there was no error, Done(nil) needs to be called.
	Done(error)
}

// ResyncEvent is used to define the data type for the resync channel
// (<resyncChan> from KeyValProtoWatcher.Watch).
type ResyncEvent interface {
	CallbackResult
	// GetValues returns key-value pairs sorted by key prefixes
	// (<keyPrefix> variable list from KeyValProtoWatcher.Watch).
	GetValues() map[string]KeyValIterator
}

// ChangeEvent is used to define the data type for the change channel
// (<changeChan> from KeyValProtoWatcher.Watch).
// A data change event contains a key identifying where the change happened
// and two values for data stored under that key: the value *before* the change
// (previous value) and the value *after* the change (current value).
type ChangeEvent interface {
	CallbackResult
	// GetChanges returns list of changes for this change event.
	GetChanges() []ProtoWatchResp
}

// KeyValIterator is an iterator for KeyVal.
type KeyValIterator interface {
	// GetNext retrieves the next value from the iterator context. The retrieved
	// value is unmarshaled and returned as <kv>. The allReceived flag is
	// set to true on the last KeyVal pair in the context.
	GetNext() (kv KeyVal, allReceived bool)
}

// ProtoWatchResp contains changed value.
type ProtoWatchResp interface {
	ChangeValue
	WithPrevValue
}

// ChangeValue represents a single propagated change.
type ChangeValue interface {
	WithChangeType
	KeyVal
}

// WithChangeType is a simple helper interface embedded by all interfaces that
// require access to change type information.
// The intent is to ensure that the same method declaration is used in different
// interfaces (composition of interfaces).
type WithChangeType interface {
	GetChangeType() Op
}

// KeyVal represents a single key-value pair.
type KeyVal interface {
	WithKey
	LazyValue
	WithRevision
}

// WithKey is a simple helper interface embedded by all interfaces that require
// access to the key of the key-value pair.
// The intent is to ensure that the same method declaration is used in different
// interfaces (composition of interfaces).
type WithKey interface {
	// GetKey returns the key of the pair
	GetKey() string
}

// WithRevision is a simple helper interface embedded by all interfaces that
// require access to the value revision.
// The intent is to ensure that the same method declaration is used in different
// interfaces (composition of interfaces).
type WithRevision interface {
	// GetRevision gets revision of current value
	GetRevision() int64
}

// WithPrevValue is a simple helper interface embedded by all interfaces that
// require access to the previous value.
// The intent is to ensure that the same method declaration is used in different
// interfaces (composition of interfaces).
type WithPrevValue interface {
	// GetPrevValue gets the previous value in the data change event.
	// The caller must provide an address of a proto message buffer
	// as <prevValue>.
	// returns:
	// - <prevValueExist> flag is set to 'true' if previous value does exist
	// - error if <prevValue> can not be properly filled
	GetPrevValue(prevValue proto.Message) (prevValueExist bool, err error)
}

// LazyValue defines value that is unmarshaled into proto message on demand.
// The reason for defining interface with only one method is primarily to unify
// interfaces in this package.
type LazyValue interface {
	// GetValue gets the current value in the data change event.
	// The caller must provide an address of a proto message buffer
	// as <value>.
	// returns:
	// - error if value argument can not be properly filled.
	GetValue(value proto.Message) error
}
