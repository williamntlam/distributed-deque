package distributeddeque

import "errors"

var (
	ErrEmpty = errors.New("distributeddeque: empty")
	ErrClosed = errors.New("distributeddeque: closed")
)

func isEmpty(err error) bool {
	return errors.Is(err, ErrEmpty)
}

func isClosed(err error) bool {
	return errors.Is(err, ErrClosed)
}