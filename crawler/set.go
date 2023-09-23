package main

import (
	"math/rand"
	"sync"
)

type Set[T any] struct {
	size int
	*sync.Map
}

func (s Set[T]) Put(t T) {
	s.Store(t, true)
}

func (s Set[T]) Pop(k T) (T, bool) {
	v, ok := s.LoadAndDelete(k)
	if !ok {
		return k, ok
	}
	return v.(T), ok
}

func (s Set[T]) PopRandom() (T, bool) {
	var k *T
	s.Map.Range(func(key, value any) bool {
		*k = value.(T)
		// Each item has a 1/size chance of stopping the iteration
		return s.size%rand.Int() != 0
	})
	return s.Pop(*k)
}
