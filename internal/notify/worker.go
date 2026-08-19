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
	// NotifyDegraded publishes what is known about a round whose summary cannot
	// be read, so it stops holding up the rounds behind it.
	NotifyDegraded(ctx context.Context, round types.Round) error
}

type Worker struct {
	logger      *slog.Logger
	store       store.Store
	retryWindow time.Duration

	notifiers map[string]Notifier
}

func NewWorker(logger *slog.Logger, stateStore store.Store, retryWindow time.Duration) *Worker {
	return &Worker{
		logger:      logger,
		store:       stateStore,
		retryWindow: retryWindow,
		notifiers:   make(map[string]Notifier),
	}
}

func (w *Worker) Register(serverID string, notifier Notifier) {
	w.notifiers[serverID] = notifier
}

func (w *Worker) Watch(ctx context.Context) error {
	for ctx.Err() == nil {
		rounds, err := w.store.UnpublishedRounds()
		if err == nil {
			for serverID, serverRounds := range rounds {
				w.publishServer(ctx, serverID, serverRounds)
			}
		}

		time.Sleep(time.Second)
	}
	return ctx.Err()
}

// publishServer walks one server's rounds oldest first and stops at the first
// round it cannot publish, so a later round can never take an earlier round's
// place in the channel.
func (w *Worker) publishServer(ctx context.Context, serverID string, rounds []types.Round) {
	notifier, ok := w.notifiers[serverID]
	if !ok {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"notify: didnt find handler",
			applog.ServerID(serverID),
		)
		return
	}

	for _, round := range rounds {
		if !w.publishRound(ctx, notifier, round) {
			return
		}
	}
}

// publishRound reports whether the walk may continue to the next round.
func (w *Worker) publishRound(ctx context.Context, notifier Notifier, round types.Round) bool {
	err := notifier.Notify(ctx, round)
	if err == nil {
		return w.markPublished(ctx, round)
	}

	w.logger.LogAttrs(
		ctx, slog.LevelError,
		"notify: failed to notify",
		applog.ServerID(round.ServerID),
		applog.RoundID(round.RoundID),
		applog.Error(err),
	)

	if !w.givingUp(round) {
		if err := w.store.RecordFailure(round.ServerID, round.RoundID); err != nil {
			w.logger.LogAttrs(
				ctx, slog.LevelError,
				"notify: failed to record failure",
				applog.ServerID(round.ServerID),
				applog.RoundID(round.RoundID),
				applog.Error(err),
			)
		}
		return false
	}

	// Out of time: publish what we have so the rounds behind this one can go
	// out. If even that fails the round keeps blocking, which is correct -
	// Discord itself is unreachable.
	if err := notifier.NotifyDegraded(ctx, round); err != nil {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"notify: failed to notify degraded",
			applog.ServerID(round.ServerID),
			applog.RoundID(round.RoundID),
			applog.Error(err),
		)
		return false
	}

	return w.markPublished(ctx, round)
}

func (w *Worker) givingUp(round types.Round) bool {
	return !round.FirstFailedAt.IsZero() && time.Since(round.FirstFailedAt) > w.retryWindow
}

// markPublished reports whether the walk may continue: a message that went out
// but was not recorded will be sent again on the next pass, so stop rather than
// stack more rounds on top of a duplicate.
func (w *Worker) markPublished(ctx context.Context, round types.Round) bool {
	err := w.store.MarkPublished(round.ServerID, round.RoundID)
	if err != nil {
		w.logger.LogAttrs(
			ctx, slog.LevelError,
			"notify: failed to record publication",
			applog.ServerID(round.ServerID),
			applog.RoundID(round.RoundID),
			applog.Error(err),
		)
		return false
	}
	return true
}
