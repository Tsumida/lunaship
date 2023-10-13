package fsm

import (
	"context"

	"github.com/samber/mo"
)

type Resource interface {
	// Global ID
	ID() string
	ResourceType() mo.Option[string]
	ToByte() mo.Result[[]byte]
	FromByte([]byte) mo.Result[Resource]
}

type ResourceLoader interface {
	// Get a newest data with ts > current
	Get(ctx context.Context, id string) mo.Result[Resource]

	// Atomic update.
	Put(ctx context.Context, ts uint64) mo.Result[Resource]
}
