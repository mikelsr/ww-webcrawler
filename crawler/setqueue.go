package main

import (
	"errors"
	"sync"
)

var (
	errQueueFull      = errors.New("queue full")
	errQueueEmpty     = errors.New("queue empty")
	errQueueDuplicate = errors.New("duplicated item")
)

// UniqueQueue is a queue that only accepts values it doesn't contain.
// Practically a RingBuffer that checks for duplicates.
type UniqueQueue[T comparable] struct {
	capacity int  // maximum capacity of the queue.
	size     *int // current size of the queue.
	head     *int // index in which the last read operation was performed.
	tail     *int // index in which the last write operation was performed.
	items    []T
	mut      *sync.Mutex
}

// Build a queue with a maximum capacity.
func NewUniqueQueue[T comparable](capacity int) UniqueQueue[T] {
	var r, w, size int = capacity - 1, capacity - 1, 0
	return UniqueQueue[T]{
		capacity: capacity,
		head:     &r,
		tail:     &w,
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
		return nilT, errQueueEmpty
	}
	// circular indexing.
	if *u.head < u.capacity-1 {
		*u.head++
	} else {
		*u.head = 0
	}
	u.decSize()
	return u.items[*u.head], nil
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
		return errQueueFull
	}

	// check for duplicates on the active range.
	if *u.head <= *u.tail {
		for i := *u.head; i < *u.tail; i++ {
			if u.items[i] == t {
				return errQueueDuplicate
			}
		}
	} else {
		for i := *u.tail; i < u.capacity; i++ {
			if u.items[i] == t {
				return errQueueDuplicate
			}
		}
		for i := 0; i < *u.head; i++ {
			if u.items[i] == t {
				return errQueueDuplicate
			}
		}
	}

	// update write position, circular indexing.
	if *u.tail < u.capacity-1 {
		*u.tail++
	} else {
		*u.tail = 0
	}

	// put t in the queue.
	u.items[*u.tail] = t

	u.incSize()
	return nil
}

// Current size of the queue.
func (u UniqueQueue[T]) Size() int {
	u.mut.Lock()
	defer u.mut.Unlock()
	return *u.size
}

// index of item t in the queue. -1 if missing.
func (u UniqueQueue[T]) indexOf(t T) int {
	if *u.size == 0 {
		return -1
	}

	if *u.head < *u.tail {
		for i := *u.head; i < *u.tail; i++ {
			if u.items[i] == t {
				return i
			}
		}
	} else {
		for i := *u.head; i < u.capacity; i++ {
			if u.items[i] == t {
				return i
			}
		}
		for i := 0; i < *u.tail; i++ {
			if u.items[i] == t {
				return i
			}
		}
	}
	return -1
}

// Remove t from the queue.
func (u UniqueQueue[T]) Remove(t T) {
	u.mut.Lock()
	defer u.mut.Unlock()
	u.remove(t)
}

// thread-unsafe implementation of remove.
func (u UniqueQueue[T]) remove(t T) {
	i := u.indexOf(t)
	// position() implicitly checks the empty case.
	if i == -1 {
		return
	}
	// no circularity.
	if *u.head < *u.tail {
		for j := i + 1; j <= *u.tail; j++ {
			u.items[j-1] = u.items[j]
		}
	} else { // circularity, removed item is at the back-end of the underlying slice.
		// shift items after the removed item one position to the front.
		if i >= *u.head {
			// back-end.
			for j := i + 1; j < u.capacity; j++ {
				u.items[j-1] = u.items[j]
			}
			// fron-end.
			for j := 0; j <= *u.tail; j++ {
				// bring item at index 0 to last index.
				if j == 0 {
					u.items[u.capacity-1] = u.items[0]
				} else {
					u.items[j-1] = u.items[j]
				}
			}
		} else if i <= *u.tail {
			// circularity, removed item is at the front-end of the underlying slice.
			// same as front-end section of the loop above.
			for j := i; j <= *u.tail; j++ {
				if j == 0 {
					u.items[u.capacity-1] = u.items[0]
				} else {
					u.items[j-1] = u.items[j]
				}
			}
		} else {
			panic("should never reach here")
		}

	}

	// Move read index back. Edge case where removed item was last item in a full queue.
	if i == *u.head {
		if *u.head <= 0 {
			*u.head = u.capacity - 1
		} else {
			*u.head--
		}
	}

	// Move write index back.
	if *u.tail <= 0 {
		*u.tail = u.capacity - 1
	} else {
		*u.tail--
	}
	u.decSize()
}

// Calculate the new size of the queue, defaulting to 0 if r/w indices match.
func (u UniqueQueue[T]) decSize() {
	if *u.head == *u.tail {
		*u.size = 0
	} else if *u.head <= *u.tail {
		*u.size = *u.tail - *u.head
	} else {
		*u.size = u.capacity + *u.tail - *u.head
	}
}

// Calculate the new size of the queue, defaulting to full if r/w indices match.
func (u UniqueQueue[T]) incSize() {
	if *u.head == *u.tail {
		*u.size = u.capacity
	} else if *u.head <= *u.tail {
		*u.size = *u.tail - *u.head
	} else {
		*u.size = u.capacity + *u.tail - *u.head + 1
	}
}
