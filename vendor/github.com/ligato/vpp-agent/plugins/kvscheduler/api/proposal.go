package api

import (
	"github.com/gogo/protobuf/proto"
)

//
// type KVDescriptor struct {
//  ...
//  // Dependencies is a slice of all possible dependencies for values under
//  // this descriptor.
//  Dependencies []Dependency
//
//  // Attributes is a slice of all attributes recognized for values under this
//  // descriptor.
//  Attributes []Attribute
// 	...
// }
//

// Attribute represents a field or property of a value. The attribute instances
// should be solely inferred from the value content itself (through the provided
// callback Get) and cannot be changed directly via NB transaction.
// While the state of attributes and their existence is bound to the state of
// the source value, they are allowed to have their own descriptors or even none
// at all.
//
// Typically, attribute represents value property (that other kv pairs may
// depend on), or extra actions taken when additional dependencies are met,
// but otherwise not blocking the base value from being added.
//
// It is not allowed for an attribute to have its own attributes - i.e. there is
// only one level of attributes.
//
// Attribute variants:
//  - singleton: 0/1 instances, empty instance name
//  - multiple instances: 0..n instances, unique instance names
type Attribute struct {
	AttributeMeta

	// Get returns a slice of all instances of this attribute for the given value.
	Get func(value proto.Message) []AttributeInstance
}

// AttributeMeta represents the static-part of the attribute definition.
type AttributeMeta struct {
	// Name of the attribute.
	Name string
	// Description can provide detailed human-readable description
	// of the attribute.
	Description string
	// Singleton flags attributes with at most one instance.
	Singleton bool
}

// AttributeInstance represent an attribute instance specific to a given value.
type AttributeInstance struct {
	// Name of the attribute instance.
	// Leave empty for singleton.
	Name string

	// AttrVal is attribute value.
	AttrVal proto.Message
}

// Dependency references another key-value pair that must exist before
// the associated value can be added.
type Dependency2 struct {
	DependencyMeta

	// Get returns selector specifically selecting which key-value pairs must
	// exist for the given value to have this dependency satisfied.
	// For value that does not have this dependency, return <hasThisDep> as false.
	Get func(value proto.Message) (selector DependencySelector, hasThisDep bool)
}

// DependencyMeta represents the static-part of the dependency definition.
type DependencyMeta struct {
	// Label should be a short, ideally human-readable, string labeling
	// the dependency.
	// Must be unique in the list of dependencies for a value.
	Label string

	// Description can provide detailed human-readable description
	// of the dependency.
	Description string
}

// DependencySelector specifically selects which key-value pair must exist
// for a given value to have the associated dependency satisfied.
type DependencySelector struct {
	// Key of another kv pair that the associated value depends on.
	// If empty, AnyOf must be defined instead.
	Key string

	// AnyOf, if not nil, must return true for at least one of the already added
	// keys for the dependency to be considered satisfied.
	// Either Key or AnyOf should be defined, but not both at the same time.
	// Note: AnyOf comes with more overhead than a static key dependency,
	// so prefer to use the latter whenever possible.
	AnyOf KeySelector
}

//////////////////////////////////////////////

//
// type KVScheduler interface {
//  ...
//  // DumpGraphSchema returns static schema for all registered value and
//  // attribute types. Once all descriptors have been registered (post-Init),
//  // the schema remains the same and therefore it should be enough to dump
//  // it at most once.
//  DumpGraphSchema() GraphSchema
//  ...
// }
//

// CommonSchema represents part of the schema shared between values and attributes.
type CommonSchema struct {
	// DescriptorName is the name of the associated descriptor.
	// Empty for attributes without descriptor.
	DescriptorName string

	// ValueDependencies provides static information about all possible
	// dependencies for the described item type.
	ValueDependencies []DependencyMeta

	// DumpDependencies is a list of descriptors that have to be dumped
	// before the associated descriptor.
	DumpDependencies []string
}

// AttributeSchema provides static information about an attribute and its
// relations with other items.
type AttributeSchema struct {
	CommonSchema
	// TODO: ModelAttribute
}

// ModelSchema provides static information about a model (value type) and its
// relations with other items.
type ModelSchema struct {
	CommonSchema
	// TODO ModelSpec

	// Attributes provides static information about all possible attribute types
	// for this model.
	Attributes []AttributeSchema
}

// GraphSchema provides static information about all types of registered items
// and their relations.
type GraphSchema struct {
	// Models is a list of all models with registered descriptors.
	Models []ModelSchema
}
