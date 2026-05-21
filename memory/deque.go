package memory

import "sync"

type MemoryDeque struct {
	mu sync.Mutex
	head *node
	tail *node
	size int64
	closed bool
}

func NewMemoryDeque() *MemoryDeque {
	return &MemoryDeque{}
}