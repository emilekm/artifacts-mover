package notify

import (
	"context"
	"log/slog"
	"time"

	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
)

type Notifier interface {
	Notify(ctx context.Context, round types.Round) error
}

type Worker struct {
	logger *slog.Logger
	store  store.Store

	notifiers map[string]Notifier
}

func NewWorker(logger *slog.Logger, stateStore store.Store) *Worker {
	return &Worker{
		logger:    logger,
		store:     stateStore,
		notifiers: make(map[string]Notifier),
	}
}

func (w *Worker) Register(serverID string, notifier Notifier) {
	w.notifiers[serverID] = notifier
}

func (w *Worker) Watch(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			rounds, err := w.store.PendingNotifications()
			if err == nil {
				for _, round := range rounds {
					w.notifyRound(ctx, round)
				}
			}

			time.Sleep(time.Second)
		}
	}
}

func (w *Worker) notifyRound(ctx context.Context, round types.Round) {
	notifier, ok := w.notifiers[round.ServerID]
	if !ok {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"notify: didnt find handler",
			applog.ServerID(round.ServerID),
		)
		return
	}

	err := notifier.Notify(ctx, round)
	if err != nil {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"notify: failed to notify",
			applog.ServerID(round.ServerID),
			applog.RoundID(round.RoundID),
			applog.Error(err),
		)
		return
	}

	err = w.store.MarkNotified(round.ServerID, round.RoundID)
	if err != nil {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"uploader: failed to round upload",
			applog.ServerID(round.ServerID),
			applog.RoundID(round.RoundID),
			applog.Error(err),
		)
	}
}
