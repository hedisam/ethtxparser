package ringbuffer

type RingBuffer[T any] struct {
	buf  []T
	head int
	tail int
	size int
}

// New creates a RingBuffer with the given capacity.
// A default capacity of 1 is used of the given value is zero.
func New[T any](capacity uint) *RingBuffer[T] {
	return &RingBuffer[T]{
		buf: make([]T, max(1, capacity)),
	}
}

// Size returns the number of elements currently in the buffer.
func (r *RingBuffer[T]) Size() int {
	return r.size
}

// IsFull returns true if the queue is full.
func (r *RingBuffer[T]) IsFull() bool {
	return r.size == cap(r.buf)
}

// Push adds the provided item to the buffer. It returns false if the queue is full and a push cannot be done.
func (r *RingBuffer[T]) Push(item T) bool {
	if r.size == cap(r.buf) {
		return false
	}

	r.buf[r.tail] = item
	r.tail = (r.tail + 1) % cap(r.buf)
	r.size++
	return true
}

// Pop removes and returns the oldest item. If empty, it returns (nil[T], false).
func (r *RingBuffer[T]) Pop() (T, bool) {
	var zero T
	if r.size == 0 {
		return zero, false
	}

	item := r.buf[r.head]
	r.buf[r.head] = zero
	r.head = (r.head + 1) % cap(r.buf)
	r.size--
	return item, true
}

// Back returns the newest block without removing it. If empty, returns (nil, false).
func (r *RingBuffer[T]) Back() (T, bool) {
	var zero T
	if r.size == 0 {
		return zero, false
	}
	idx := (r.tail - 1 + cap(r.buf)) % cap(r.buf)
	return r.buf[idx], true
}

// DropBack discards the newest item from the buffer (if any) without returning it.
func (r *RingBuffer[T]) DropBack() {
	if r.size == 0 {
		return
	}
	// move tail back to the last item
	r.tail = (r.tail - 1 + cap(r.buf)) % cap(r.buf)

	var zero T
	r.buf[r.tail] = zero
	r.size--
}
