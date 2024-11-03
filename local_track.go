package moqtransport

import (
	"context"
	"iter"

	"github.com/mengelbart/moqtransport/internal/container"
)

var _ LocalTrack = (*container.ArrayList[Object])(nil)

type LocalTrack interface {
	IterUntilClosed(context.Context) iter.Seq[Object]
	Close()
}

type ListTrack struct {
	list   *container.ArrayList[Object]
	nextID uint64
}

func NewListTrack() *ListTrack {
	return &ListTrack{
		list:   container.NewArrayList[Object](),
		nextID: 0,
	}
}

func (t *ListTrack) Append(o Object) {
	t.list.Append(o)
}

func (t *ListTrack) IterUntilClosed(ctx context.Context) iter.Seq[Object] {
	return t.list.IterUntilClosed(ctx)
}

func (l *ListTrack) Close() {
	l.list.Close()
}
