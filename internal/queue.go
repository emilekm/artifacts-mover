package internal

import (
	"sync"
)

type queueItem struct {
	Next *queueItem
	Fn   func()
}

type Queue struct {
	mutex sync.Mutex
	first *queueItem
	last  *queueItem
}

func NewQueue() *Queue {
	return &Queue{}
}

func (q *Queue) Add(fn func()) {
	q.insertItem(&queueItem{
		Fn: fn,
	})
}

func (q *Queue) insertItem(item *queueItem) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.first == nil {
		q.first = item
		q.last = item
		q.start()
		return
	}

	q.last.Next = item
	q.last = item
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

			q.first.Fn()

			q.mutex.Lock()
			q.first = q.first.Next
			if q.first == nil {
				q.last = nil
			}
			q.mutex.Unlock()
		}
	}()
}
