package notify

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/jobs"
	applog "github.com/emilekm/artifacts-mover/internal/log"
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

	allUploaded := true
	for _, artifact := range round.Artifacts {
		if !artifact.Uploaded {
			allUploaded = false
			break
		}
	}

	if allUploaded {
		go func(round types.Round) {
			for _, artifact := range round {
				err := os.Remove(artifact.Path)
				if err != nil {
					w.logger.LogAttrs(
						context.TODO(), slog.LevelError,
						"notify: failed to remove file",
						applog.Path(artifact.Path),
						applog.Error(err),
					)
				}
			}
		}(round.ArtifactsByType)
	}

	return nil
}
