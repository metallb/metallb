package client

import (
	"context"

	"github.com/gogo/protobuf/proto"
	api "github.com/ligato/vpp-agent/api/genericmanager"
)

// ConfigClient defines the client-side interface for config.
type ConfigClient interface {
	// KnownModels retrieves list of known modules.
	KnownModels() ([]api.ModelInfo, error)

	// ChangeRequest returns transaction for changing config.
	ChangeRequest() ChangeRequest

	// ResyncConfig overwrites existing config.
	ResyncConfig(items ...proto.Message) error

	// GetConfig retrieves current config into dsts.
	GetConfig(dsts ...interface{}) error
}

// ChangeRequest is interface for config change request.
type ChangeRequest interface {
	// Update appends updates for given items to the request.
	Update(items ...proto.Message) ChangeRequest

	// Delete appends deletes for given items to the request.
	Delete(items ...proto.Message) ChangeRequest

	// Send sends the request.
	Send(ctx context.Context) error
}
