package internal

import (
	"context"
	"sync"
)

type queueItem struct {
	Next *queueItem
	Ch   chan error
	Fn   func() error
}

type Queue struct {
	mutex  sync.Mutex
	first  *queueItem
	cancel context.CancelFunc
}

func NewQueue() *Queue {
	return &Queue{}
}

func (q *Queue) Add(fn func() error) chan error {
	newItem := &queueItem{
		Fn: fn,
		Ch: make(chan error, 1),
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.first == nil {
		q.first = newItem
	} else {
		last := q.first
		for {
			if last.Next == nil {
				break
			}
			last = last.Next
		}
		last.Next = newItem
	}

	if q.cancel == nil {
		ctx, cancel := context.WithCancel(context.Background())
		q.cancel = cancel
		go q.Start(ctx)
	}

	return newItem.Ch
}

func (q *Queue) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			q.cancel = nil
			return
		default:
			q.mutex.Lock()
			if q.first == nil {
				q.cancel()
				q.cancel = nil
				return
			}
			q.mutex.Unlock()

			err := q.first.Fn()
			if err != nil {
				q.first.Ch <- err
			}
			close(q.first.Ch)

			q.mutex.Lock()
			q.first = q.first.Next
			q.mutex.Unlock()
		}
	}
}
