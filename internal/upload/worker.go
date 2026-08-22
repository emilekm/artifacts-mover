package upload

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/notify"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
	"gorm.io/gorm"
)

type UploadArgs struct {
	ArtifactID uint
}

func (UploadArgs) Kind() string { return "upload" }

type Uploader interface {
	Upload(ctx context.Context, path string, typ types.ArtifactType) error
}

type Worker struct {
	river.WorkerDefaults[UploadArgs]

	logger *slog.Logger
	db     *db.DB

	uploaders map[string]Uploader
}

func NewWorker(logger *slog.Logger, db *db.DB) *Worker {
	return &Worker{
		logger:    logger,
		db:        db,
		uploaders: make(map[string]Uploader),
	}
}

func (w *Worker) Register(serverID string, uploader Uploader) {
	w.uploaders[serverID] = uploader
}

func (w *Worker) Work(ctx context.Context, job *river.Job[UploadArgs]) error {
	artifactID := job.Args.ArtifactID
	artifact, err := gorm.G[db.Artifact](w.db.DB).Where("id = ?", artifactID).Preload("Round", nil).First(ctx)
	if err != nil {
		return err
	}

	uploader, ok := w.uploaders[artifact.Round.ServerID]
	if !ok {
		// w.logger.LogAttrs(
		// 	ctx, slog.LevelError,
		// 	"upload: didnt find handler",
		// 	applog.ServerID(serverID),
		// )
		return fmt.Errorf("upload_no_handler")
	}

	err = uploader.Upload(ctx, artifact.Path, artifact.Type)
	if err != nil {
		return err
	}

	tx := w.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}
	defer tx.Rollback()

	sqlTx := tx.Statement.Statement.ConnPool.(*sql.Tx)
	client := river.ClientFromContext[*sql.Tx](ctx)

	_, err = gorm.G[db.Artifact](tx).Where("id = ?", artifactID).Update(ctx, "uploaded", true)
	if err != nil {
		return err
	}

	_, err = client.InsertTx(ctx, sqlTx, notify.SyncNotificationArgs{
		RoundID: artifact.RoundID,
	}, nil)
	if err != nil {
		return err
	}

	tx.Commit()
	return nil
}
