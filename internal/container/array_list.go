package container

import (
	"context"
	"iter"
	"slices"
	"sync"
	"sync/atomic"
)

type rwlockwrapper struct {
	lock *sync.RWMutex
}

func (l *rwlockwrapper) Lock() {
	l.lock.RLock()
}

func (l *rwlockwrapper) Unlock() {
	l.lock.RUnlock()
}

const defaultSize = 1024

type ArrayList[E any] struct {
	elements []E
	lock     sync.RWMutex
	cv       sync.Cond
	closed   atomic.Bool
}

func (l *ArrayList[E]) init(size int) *ArrayList[E] {
	l.elements = make([]E, 0, size)
	l.lock = sync.RWMutex{}
	l.cv = *sync.NewCond(&rwlockwrapper{
		lock: &l.lock,
	})
	l.closed = atomic.Bool{}
	return l
}

func NewArrayListWithSize[E any](size int) *ArrayList[E] {
	return new(ArrayList[E]).init(size)
}

func NewArrayList[E any]() *ArrayList[E] {
	return NewArrayListWithSize[E](defaultSize)
}

func (l *ArrayList[E]) Append(e E) {
	l.lock.Lock()
	defer l.lock.Unlock()
	l.elements = append(l.elements, e)
	l.cv.Broadcast()
}

func (l *ArrayList[E]) Close() {
	l.closed.Store(true)
	l.cv.Broadcast()
}

func (l *ArrayList[E]) BinarySearch(e E, cmp func(E, E) int) (E, bool) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	n, ok := slices.BinarySearchFunc(l.elements, e, cmp)
	if !ok {
		return *new(E), ok
	}
	return l.elements[n], ok
}

func (l *ArrayList[E]) InsertSorted(e E, cmp func(E, E) int) {
	l.lock.RLock()
	defer l.lock.RUnlock()
	n, _ := slices.BinarySearchFunc(l.elements, e, cmp)
	l.elements = append(l.elements, e)
	copy(l.elements[n+1:], l.elements[n:])
	l.elements[n] = e
	l.cv.Broadcast()
}

func (l *ArrayList[E]) IterUntilClosed(ctx context.Context) iter.Seq[E] {
	return func(yield func(E) bool) {
		i := 0
		for {
			l.lock.RLock()
			for len(l.elements) <= i && !l.closed.Load() && !ctxDone(ctx) {
				l.cv.Wait()
			}
			if l.closed.Load() && i == len(l.elements) || ctxDone(ctx) {
				l.lock.RUnlock()
				return
			}
			e := l.elements[i]
			l.lock.RUnlock()
			if !yield(e) {
				return
			}
			i++
		}
	}
}

func ctxDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
