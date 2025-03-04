package slices

import (
	"iter"
	"slices"
)

func Collect[E any](seq iter.Seq[E]) []E {
	return slices.Collect(seq)
}

func Map[K any, V any](ee []K, f func(e K) V) iter.Seq[V] {
	return func(yield func(V) bool) {
		for _, v := range ee {
			if !yield(f(v)) {
				return
			}
		}
	}
}
