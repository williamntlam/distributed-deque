package memory

type node struct {
	value []byte
	prev *node
	next *node
}