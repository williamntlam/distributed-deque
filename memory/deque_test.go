package memory_test

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"testing"

	distributeddeque "github.com/williamntlam/distributed-deque"
	"github.com/williamntlam/distributed-deque/memory"
)

func TestPopFront_EmptyReturnsErrEmpty(t *testing.T) {
	d := memory.NewMemoryDeque()

	_, err := d.PopFront(context.Background())
	if !errors.Is(err, distributeddeque.ErrEmpty) {
		t.Fatalf("PopFront() error = %v, want ErrEmpty", err)
	}
}

func TestPopBack_EmptyReturnsErrEmpty(t *testing.T) {
	d := memory.NewMemoryDeque()

	_, err := d.PopBack(context.Background())
	if !errors.Is(err, distributeddeque.ErrEmpty) {
		t.Fatalf("PopBack() error = %v, want ErrEmpty", err)
	}
}

func TestLen_Empty(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	n, err := d.Len(ctx)
	if err != nil {
		t.Fatalf("Len() error = %v", err)
	}
	if n != 0 {
		t.Fatalf("Len() = %d, want 0", n)
	}
}

func TestPushBack_PopFront_FIFO(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	for _, v := range [][]byte{[]byte("a"), []byte("b"), []byte("c")} {
		if err := d.PushBack(ctx, v); err != nil {
			t.Fatalf("PushBack(%q) error = %v", v, err)
		}
	}

	want := [][]byte{[]byte("a"), []byte("b"), []byte("c")}
	for i, w := range want {
		got, err := d.PopFront(ctx)
		if err != nil {
			t.Fatalf("PopFront #%d error = %v", i, err)
		}
		if !bytes.Equal(got, w) {
			t.Fatalf("PopFront #%d = %q, want %q", i, got, w)
		}
	}

	n, err := d.Len(ctx)
	if err != nil {
		t.Fatalf("Len() error = %v", err)
	}
	if n != 0 {
		t.Fatalf("Len() after drain = %d, want 0", n)
	}

	_, err = d.PopFront(ctx)
	if !errors.Is(err, distributeddeque.ErrEmpty) {
		t.Fatalf("PopFront() on empty deque error = %v, want ErrEmpty", err)
	}
}

func TestPushFront_PopBack_LIFOAtBack(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	for _, v := range [][]byte{[]byte("first"), []byte("second")} {
		if err := d.PushFront(ctx, v); err != nil {
			t.Fatalf("PushFront(%q) error = %v", v, err)
		}
	}

	// head -> second, first -> tail
	got, err := d.PopBack(ctx)
	if err != nil {
		t.Fatalf("PopBack() error = %v", err)
	}
	if !bytes.Equal(got, []byte("first")) {
		t.Fatalf("PopBack() = %q, want first", got)
	}

	got, err = d.PopBack(ctx)
	if err != nil {
		t.Fatalf("PopBack() error = %v", err)
	}
	if !bytes.Equal(got, []byte("second")) {
		t.Fatalf("PopBack() = %q, want second", got)
	}
}

func TestPushBothEnds_PopBothEnds(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	if err := d.PushBack(ctx, []byte("tail")); err != nil {
		t.Fatalf("PushBack error = %v", err)
	}
	if err := d.PushFront(ctx, []byte("head")); err != nil {
		t.Fatalf("PushFront error = %v", err)
	}

	n, err := d.Len(ctx)
	if err != nil || n != 2 {
		t.Fatalf("Len() = (%d, %v), want (2, nil)", n, err)
	}

	front, err := d.PopFront(ctx)
	if err != nil || !bytes.Equal(front, []byte("head")) {
		t.Fatalf("PopFront() = (%q, %v), want (head, nil)", front, err)
	}

	back, err := d.PopBack(ctx)
	if err != nil || !bytes.Equal(back, []byte("tail")) {
		t.Fatalf("PopBack() = (%q, %v), want (tail, nil)", back, err)
	}
}

func TestClose_RejectsFurtherOps(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	if err := d.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := d.PushBack(ctx, []byte("x")); !errors.Is(err, distributeddeque.ErrClosed) {
		t.Fatalf("PushBack after Close error = %v, want ErrClosed", err)
	}

	_, err := d.PopFront(ctx)
	if !errors.Is(err, distributeddeque.ErrClosed) {
		t.Fatalf("PopFront after Close error = %v, want ErrClosed", err)
	}

	_, err = d.Len(ctx)
	if !errors.Is(err, distributeddeque.ErrClosed) {
		t.Fatalf("Len after Close error = %v, want ErrClosed", err)
	}
}

func TestClose_Idempotent(t *testing.T) {
	d := memory.NewMemoryDeque()

	if err := d.Close(); err != nil {
		t.Fatalf("first Close() error = %v", err)
	}
	if err := d.Close(); err != nil {
		t.Fatalf("second Close() error = %v, want nil", err)
	}
}

func TestConcurrentPushPop(t *testing.T) {
	d := memory.NewMemoryDeque()
	ctx := context.Background()

	const goroutines = 32
	const perG = 100

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	for g := range goroutines {
		go func() {
			defer wg.Done()
			for i := range perG {
				_ = d.PushBack(ctx, []byte{byte(i)})
			}
		}()

		go func() {
			defer wg.Done()
			for range perG {
				_, _ = d.PopFront(ctx)
			}
		}()
	}

	wg.Wait()

	// No assertion on final Len — pops may win the race — but must not panic or race.
	n, err := d.Len(ctx)
	if err != nil {
		t.Fatalf("Len() error = %v", err)
	}
	if n < 0 {
		t.Fatalf("Len() = %d, want >= 0", n)
	}
}
