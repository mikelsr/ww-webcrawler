package main

import (
	"sync"
	"sync/atomic"
	"time"
)

type TimedSet[T any] struct {
	size *atomic.Int32
	sm   *sync.Map
}

func NewTimedSet[T any]() TimedSet[T] {
	return TimedSet[T]{
		size: &atomic.Int32{},
		sm:   &sync.Map{},
	}
}

func (s TimedSet[T]) Put(t T) {
	_, ok := s.sm.Load(t)
	if !ok {
		s.size.Add(1)
	}
	s.sm.Store(t, time.Now())
}

func (s TimedSet[T]) Pop(k T) (T, bool) {
	_, ok := s.sm.LoadAndDelete(k)
	if ok {
		s.size.Add(-1)
	}
	return k, ok
}

func (s TimedSet[T]) Has(t T) bool {
	_, ok := s.sm.Load(t)
	return ok
}

func (s TimedSet[T]) PopOldest() (T, bool) {
	var k *T

	// start with the latest time possible
	faraway := time.Unix(1<<63-1, 0)
	var oldest = &faraway
	// find the oldest value in the set
	s.sm.Range(func(key, value any) bool {
		if value.(time.Time).Before(*oldest) {
			*k = key.(T)
			*oldest = value.(time.Time)
		}
		return true
	})
	// return empty value
	if k == nil {
		var t T
		return s.Pop(t)
	}
	return s.Pop(*k)
}

func (s TimedSet[T]) Range(f func(key, value any) bool) {
	s.sm.Range(f)
}

func (s TimedSet[T]) Size() int {
	return int(s.size.Load())
}
