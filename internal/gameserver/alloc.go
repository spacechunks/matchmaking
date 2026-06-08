package gameserver

import "context"

type Allocator interface {
	AllocateInstance(ctx context.Context) (string, error)
}
