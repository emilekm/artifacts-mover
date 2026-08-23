package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/jobs"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
)

type Notifier interface {
	Notify(ctx context.Context, msgID *string, artifacts types.Round) (string, error)
}

type Worker struct {
	river.WorkerDefaults[jobs.SyncNotificationArgs]

	logger *slog.Logger
	db     *db.DB

	notifiers map[string]Notifier
}

func NewWorker(logger *slog.Logger, db *db.DB) *Worker {
	return &Worker{
		logger:    logger,
		db:        db,
		notifiers: make(map[string]Notifier),
	}
}

func (w *Worker) Register(serverID string, notifier Notifier) {
	w.notifiers[serverID] = notifier
}

func (w *Worker) Work(ctx context.Context, job *river.Job[jobs.SyncNotificationArgs]) error {
	roundID := job.Args.RoundID
	round, err := w.db.Round(ctx, roundID)
	if err != nil {
		return err
	}

	notifier, ok := w.notifiers[round.ServerID]
	if !ok {
		return fmt.Errorf("notify_no_handler")
	}

	msgID, err := notifier.Notify(ctx, round.DiscordMessageID, round.ArtifactsByType)
	if err != nil {
		return err
	}

	if round.DiscordMessageID == nil || msgID != *round.DiscordMessageID {
		err = w.db.UpdateMessageID(ctx, roundID, msgID)
		if err != nil {
			return err
		}
	}

	return nil
}
