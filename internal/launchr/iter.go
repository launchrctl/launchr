package launchr

import "iter"

// SliceSeqStateful allows for resuming iteration after slice grows.
type SliceSeqStateful[V any] struct {
	slice *[]V
	index int // index keeps track of where we are in slice.
}

// NewSliceSeqStateful creates a new [SliceSeqStateful].
func NewSliceSeqStateful[V any](slice *[]V) *SliceSeqStateful[V] {
	return &SliceSeqStateful[V]{slice: slice}
}

// Seq returns a slice iterator.
func (seq *SliceSeqStateful[V]) Seq() iter.Seq[V] {
	return func(yield func(V) bool) {
		for seq.index < len(*seq.slice) {
			if !yield((*seq.slice)[seq.index]) {
				// Early return by user break.
				seq.index++
				return
			}
			seq.index++
		}
		// If iteration exhausts naturally, it stays exhausted until slice grows again
	}
}

// Reset resets current index to start.
func (seq *SliceSeqStateful[V]) Reset() {
	seq.index = 0
}
