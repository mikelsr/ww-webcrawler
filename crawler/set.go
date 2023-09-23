package main

import (
	"math/rand"
	"sync"
	"sync/atomic"
)

type Set[T any] struct {
	size *atomic.Int32
	sm   *sync.Map
}

func NewSet[T any]() Set[T] {
	return Set[T]{
		size: &atomic.Int32{},
		sm:   &sync.Map{},
	}
}

func (s Set[T]) Put(t T) {
	_, ok := s.sm.Load(t)
	if !ok {
		s.size.Add(1)
	}
	s.sm.Store(t, true)
}

func (s Set[T]) Has(t T) bool {
	_, ok := s.sm.Load(t)
	return ok
}

func (s Set[T]) Pop(k T) (T, bool) {
	_, ok := s.sm.LoadAndDelete(k)
	if ok {
		s.size.Add(-1)
	}
	return k, ok
}

func (s Set[T]) PopRandom() (T, bool) {
	var k *T
	size := s.size.Load()
	s.sm.Range(func(key, value any) bool {
		*k = key.(T)
		// Each item has a 1/size chance of stopping the iteration
		return size%rand.Int31() != 0
	})
	return s.Pop(*k)
}

func (s Set[T]) Range(f func(key, value any) bool) {
	s.sm.Range(f)
}

func (s Set[T]) Size() int {
	return int(s.size.Load())
}
