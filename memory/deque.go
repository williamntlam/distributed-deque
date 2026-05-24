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

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.requireOpen(); err != nil {
		return err
	}

	// Create a new node.

	newNode := &node{value: value}
	
	if d.head == nil {

		d.head = newNode
		d.tail = newNode

	} else {

		// Add node to Deque

		newNode.prev = d.tail
		d.tail.next = newNode
		d.tail = newNode

	}

	d.size++

	return nil

}

func (d *MemoryDeque) PushFront(ctx context.Context, value []byte) error {

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.requireOpen(); err != nil {
		return err
	}

	newNode := &node{value: value}

	if d.head == nil {

		d.head = newNode
		d.tail = newNode

	} else {

		newNode.next = d.head
		d.head.prev = newNode
		d.head = newNode

	}

	d.size++

	return nil

}

func (d *MemoryDeque) PopFront(ctx context.Context) ([]byte, error) {

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.requireOpen(); err != nil {
		return nil, err
	}

	if d.head == nil {
		return nil, distributeddeque.ErrEmpty
	}

	frontNode := d.head
	d.head = frontNode.next

	if d.head != nil {
		d.head.prev = nil
	} else {
		d.tail = nil
	}

	d.size--

	return frontNode.value, nil

}

func (d *MemoryDeque) PopBack(ctx context.Context) ([]byte, error) {

	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.requireOpen(); err != nil {
		return nil, err
	}

	if d.tail == nil {
		return nil, distributeddeque.ErrEmpty
	}

	backNode := d.tail
	d.tail = backNode.prev
	if d.tail != nil {
		d.tail.next = nil
	} else {
		d.head = nil
	}

	d.size -= 1

	return backNode.value, nil

}

func (d *MemoryDeque) Close() error { 

	d.mu.Lock()
	defer d.mu.Unlock()

	d.closed = true
	
	return nil
}

var _ distributeddeque.Deque = (*MemoryDeque)(nil)

func NewMemoryDeque() *MemoryDeque {
	return &MemoryDeque{}
}