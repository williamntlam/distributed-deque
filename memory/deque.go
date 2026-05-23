package memory

import (
	"context"
	"sync"
	distributeddeque "github.com/williamntlam/distributed-deque"
)

type MemoryDeque struct {
	mu sync.Mutex
	head *node
	tail *node
	size int64
	closed bool
}

func (d* MemoryDeque) requireOpen() error {
	
	if d.closed {
		return distributeddeque.ErrClosed
	}

	return nil
}
 
func (d *MemoryDeque) Len(ctx context.Context) (int64, error) {

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.requireOpen(); err != nil {
		return 0, err
	}

	return d.size, nil

}

func (d *MemoryDeque) PushBack(ctx context.Context, value []byte) error {

}

func (d *MemoryDeque) PushFront(ctx context.Context, value []byte) error {

}

func (d *MemoryDeque) PopFront(ctx context.Context) ([]byte, error) {

}

func (d *MemoryDeque) PopBack(ctx context.Context) ([]byte, error) {

}

func (d *MemoryDeque) Close() error {  }

var _ distributeddeque.Deque = (*MemoryDeque)(nil)

func NewMemoryDeque() *MemoryDeque {
	return &MemoryDeque{}
}