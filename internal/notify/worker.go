package notify

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/jobs"
	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
)

const (
	allUploadsWaitTime = 10 * time.Second
)

type Notifier interface {
	Notify(ctx context.Context, msgID *string, artifacts types.Round) (string, error)
	ReserveMessageID(ctx context.Context, timestamp time.Time) (string, error)
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
	serverID := job.Args.ServerID
	roundID := job.Args.RoundID

	notifier, ok := w.notifiers[serverID]
	if !ok {
		return fmt.Errorf("notify_no_handler")
	}

	msgID := ""
	defer func() {
		if msgID == "" {
			msgID, err := notifier.ReserveMessageID(ctx, job.Args.Timestamp)
			if err != nil {
				w.logger.LogAttrs(
					ctx, slog.LevelError,
					"notify: failed to reserver message",
					applog.Error(err),
				)
			}

			err = w.db.UpdateMessageID(ctx, roundID, msgID)
			if err != nil {
				// TODO: remove message
				w.logger.LogAttrs(
					ctx, slog.LevelError,
					"notify: update reserved message ID",
					applog.Error(err),
				)
			}
		}
	}()

	round, err := w.db.Round(ctx, roundID)
	if err != nil {
		return err
	}
	if round.DiscordMessageID != nil {
		msgID = *round.DiscordMessageID
	}

	tctx, tcancel := context.WithTimeout(ctx, allUploadsWaitTime)
	defer tcancel()
	ticker := time.NewTicker(501 * time.Millisecond)

L:
	for {
		if allArtifactsUploaded(round.ArtifactsByType) {
			break
		}

		select {
		case <-tctx.Done():
			ticker.Stop()
			break L
		case <-ticker.C:
			round, err = w.db.Round(tctx, roundID)
			if err != nil {
				w.logger.LogAttrs(
					ctx, slog.LevelError,
					"notify: failed to fetch round while waiting for uploads",
					applog.Error(err),
				)
			}
		}
	}

	msgID, err = notifier.Notify(ctx, round.DiscordMessageID, round.ArtifactsByType)
	if err != nil {
		return err
	}

	if round.DiscordMessageID == nil || msgID != *round.DiscordMessageID {
		err = w.db.UpdateMessageID(ctx, roundID, msgID)
		if err != nil {
			return err
		}
	}

	if allArtifactsUploaded(round.ArtifactsByType) {
		err = w.db.CancelWaitingSyncJobs(ctx, river.ClientFromContext[*sql.Tx](ctx), serverID, roundID)
		if err != nil {
			w.logger.LogAttrs(
				ctx, slog.LevelError,
				"notify: cancel waiting sync jobs",
				applog.Error(err),
			)
		}
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

func allArtifactsUploaded(round types.Round) bool {
	for _, artifact := range round {
		if !artifact.Uploaded {
			return false
		}
	}
	return true
}
