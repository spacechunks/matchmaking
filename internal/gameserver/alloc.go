package gameserver

import "context"

type Allocator interface {
	AllocateGameServer(ctx context.Context) (string, error)
}
