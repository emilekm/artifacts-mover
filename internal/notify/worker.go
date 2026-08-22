package notify

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"gorm.io/gorm"
)

type Notifier interface {
	Notify(ctx context.Context, msgID *string, artifacts types.Round) (string, error)
}

type SyncNotificationArgs struct {
	RoundID uint
}

func (SyncNotificationArgs) Kind() string { return "notify" }

func (SyncNotificationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStateAvailable,
				rivertype.JobStateRetryable,
			},
		},
	}
}

type Worker struct {
	river.WorkerDefaults[SyncNotificationArgs]

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

func (w *Worker) Work(ctx context.Context, job *river.Job[SyncNotificationArgs]) error {
	roundID := job.Args.RoundID
	round, err := gorm.G[db.Round](w.db.DB).Preload("Artifacts", func(db gorm.PreloadBuilder) error {
		db.Where("uploaded = ?", true)
		return nil
	}).Where("id = ?", roundID).First(ctx)
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
		_, err = gorm.G[db.Round](w.db.DB).Where("id = ?", roundID).Update(ctx, "discord_message_id", msgID)
		if err != nil {
			return err
		}
	}

	return nil
}
