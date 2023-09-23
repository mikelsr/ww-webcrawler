package main

import (
	"errors"
	"sync"
)

// UniqueQueue is a queue that only accepts values it doesn't contain.
// Practically a RingBuffer that checks for duplicates.
type UniqueQueue[T comparable] struct {
	capacity int
	size     *int
	lastR    *int
	lastW    *int
	items    []T
	mut      *sync.Mutex
}

// Build a queue with a maximum capacity.
func NewUniqueQueue[T comparable](capacity int) UniqueQueue[T] {
	var r, w, size int = -1, -1, 0
	return UniqueQueue[T]{
		capacity: capacity,
		lastR:    &r,
		lastW:    &w,
		size:     &size,
		items:    make([]T, capacity),
		mut:      &sync.Mutex{},
	}
}

// Get the oldest item in the queue.
func (u UniqueQueue[T]) Get() (T, error) {
	u.mut.Lock()
	defer u.mut.Unlock()
	return u.get()
}

// thread-unsage implementation of Get.
func (u UniqueQueue[T]) get() (T, error) {
	if *u.size == 0 {
		var nilT T
		return nilT, errors.New("queue empty")
	}
	// circular indexing.
	if *u.lastR < u.capacity-1 {
		*u.lastR++
	} else {
		*u.lastR = 0
	}
	*u.size = u.sizeAfterR()
	return u.items[*u.lastR], nil
}

// Put an item into the queue.
func (u UniqueQueue[T]) Put(t T) error {
	u.mut.Lock()
	defer u.mut.Unlock()
	return u.put(t)
}

// thread-unsafe implementation of put.
func (u UniqueQueue[T]) put(t T) error {
	if *u.size >= u.capacity {
		return errors.New("queue full")
	}

	for i, x := range u.items {
		// check only in the in-use range.
		if *u.lastR <= *u.lastW && (i < *u.lastR || i > *u.lastW) {
			continue
		} else if i < *u.lastR && i > *u.lastW {
			continue
		}
		if x == t {
			return errors.New("duplicated item")
		}
	}

	// circular indexing.
	if *u.lastW < u.capacity-1 {
		*u.lastW++
	} else {
		*u.lastW = 0
	}

	u.items[*u.lastW] = t

	*u.size = u.sizeAfterW()
	return nil
}

// Current size of the queue.
func (u UniqueQueue[T]) Size() int {
	u.mut.Lock()
	defer u.mut.Unlock()
	return *u.size
}

// calculate the size after a read.
func (u UniqueQueue[T]) sizeAfterR() int {
	// r possibly caught up with w, equality means zero size.
	if *u.lastW >= *u.lastR {
		return *u.lastW - *u.lastR
	} else {
		return u.capacity + *u.lastW - *u.lastR
	}
}

// calculate the size after a write.
func (u UniqueQueue[T]) sizeAfterW() int {
	if *u.lastW > *u.lastR {
		return *u.lastW - *u.lastR
		// w possibly caught up with r, equality means max size.
	} else {
		return u.capacity + *u.lastW - *u.lastR
	}
}
