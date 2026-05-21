package distributeddeque

import "context"

type Deque interface {
	PushFront(ctx context.Context, value[]byte) error
	PushBack(ctx context.Context, value[]byte) error
	PopFront(ctx context.Context) ([]byte, error)
	PopBack(ctx context.Context) ([]byte, error)
	Len(ctx context.Context) (int64, error)
	Close() error
}