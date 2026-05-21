package memory

import "sync"

// MemoryDeque is an in-process deque backed by a doubly-linked list (see node.go).
// Implement distributeddeque.Deque in this file when you are ready.
type MemoryDeque struct {
	mu     sync.Mutex
	head   *node
	tail   *node
	size   int
	closed bool
}

// NewMemoryDeque returns an empty deque.
func NewMemoryDeque() *MemoryDeque {
	return &MemoryDeque{}
}
