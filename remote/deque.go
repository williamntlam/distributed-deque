package remote

// RemoteDeque is a distributeddeque.Deque client that talks to cmd/queued (planned).
type RemoteDeque struct{}

// NewRemoteDeque constructs a client (not implemented).
func NewRemoteDeque() *RemoteDeque {
	return &RemoteDeque{}
}
