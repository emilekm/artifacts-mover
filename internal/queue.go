package internal

import (
	"sync"
)

type queueItem struct {
	Next *queueItem
	Ch   chan error
	Fn   func() error
}

type Queue struct {
	mutex sync.Mutex
	first *queueItem
}

func NewQueue() *Queue {
	return &Queue{}
}

func (q *Queue) Add(fn func() error) chan error {
	ch := make(chan error, 1)

	q.insertItem(&queueItem{
		Fn: fn,
		Ch: ch,
	})

	return ch
}

func (q *Queue) insertItem(item *queueItem) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.first == nil {
		q.first = item
		q.start()
		return
	}

	last := q.first
	for {
		if last.Next == nil {
			break
		}
		last = last.Next
	}
	last.Next = item
}

func (q *Queue) start() {
	go func() {
		for {
			q.mutex.Lock()
			if q.first == nil {
				q.mutex.Unlock()
				return
			}
			q.mutex.Unlock()

			q.first.Ch <- q.first.Fn()
			close(q.first.Ch)

			q.mutex.Lock()
			q.first = q.first.Next
			q.mutex.Unlock()
		}
	}()
}
