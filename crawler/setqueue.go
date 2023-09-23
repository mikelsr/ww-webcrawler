package main

import (
	"errors"
	"sync"
)

// UniqueQueue is a queue that only accepts values it doesn't contain
type UniqueQueue[T comparable] struct {
	capacity int
	lastR    int
	lastW    int
	items    []T
	mut      *sync.RWMutex
}

func NewUniqueQueue[T comparable](capacity int) UniqueQueue[T] {
	return UniqueQueue[T]{
		capacity: capacity,
		lastR:    -1,
		lastW:    -1,
		items:    make([]T, capacity),
		mut:      &sync.RWMutex{},
	}
}

func (u UniqueQueue[T]) size() int {
	if u.lastW >= u.lastR {
		return u.lastW - u.lastR
	} else {
		return u.capacity + u.lastW - u.lastR
	}
}

func (u UniqueQueue[T]) put(t T) error {
	if u.size() >= u.capacity {
		return errors.New("queue full")
	}

	for _, x := range u.items {
		if x == t {
			return errors.New("duplicated item")
		}
	}

	u.lastW++
	u.items[u.lastW] = t
	return nil
}

func (u UniqueQueue[T]) get() (T, error) {
	if u.size() == 0 {
		var nilT T
		return nilT, errors.New("queue empty")
	}
	u.lastR++
	return u.items[u.lastR], nil
}
