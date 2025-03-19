package internal

import (
	"context"
	"log/slog"
	"sync"
)

type queueItem struct {
	Next   *queueItem
	Server *Server
	Round  *Round
}

type Queue struct {
	mutex  sync.Mutex
	first  *queueItem
	cancel context.CancelFunc
}

func (q *Queue) Add(server *Server, round *Round) {
	newItem := &queueItem{
		Server: server,
		Round:  round,
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

			err := q.first.Server.Upload(q.first.Round)
			if err != nil {
				slog.Error("upload error", "error", err)
			}

			q.mutex.Lock()
			q.first = q.first.Next
			q.mutex.Unlock()
		}
	}
}
