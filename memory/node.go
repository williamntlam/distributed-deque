package memory

// node is one element in the doubly-linked list backing MemoryDeque.
type node struct {
	value []byte
	prev  *node
	next  *node
}
