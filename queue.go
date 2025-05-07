package moqtransport

import "context"

type queue[T any] struct {
	queue chan T
}

func newQueue[T any]() *queue[T] {
	return &queue[T]{
		queue: make(chan T, 1),
	}
}

// For some reason the linter thinks this is unused, but it implements the
// controlMessageQueue interface.
//
//nolint:unused
func (q *queue[T]) enqueue(ctx context.Context, msg T) error {
	select {
	case q.queue <- msg:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

// For some reason the linter thinks this is unused, but it implements the
// controlMessageQueue interface.
//
//nolint:unused
func (q *queue[T]) dequeue(ctx context.Context) (T, error) {
	select {
	case <-ctx.Done():
		return *new(T), context.Cause(ctx)
	case msg := <-q.queue:
		return msg, nil
	}
}
