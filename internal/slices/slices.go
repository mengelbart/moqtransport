package slices

import (
	"iter"
	"slices"
)

func Collect[E any](seq iter.Seq[E]) []E {
	return slices.Collect(seq)
}

func Backward[Slice ~[]E, E any](s Slice) iter.Seq2[int, E] {
	return slices.Backward(s)
}

func Contains[S ~[]E, E comparable](s S, v E) bool {
	return slices.Contains(s, v)
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
