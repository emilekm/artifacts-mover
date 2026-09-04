package upload

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/jobs"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
)

type Uploader interface {
	Upload(ctx context.Context, artifact types.Artifact) error
}

type Worker struct {
	river.WorkerDefaults[jobs.UploadArgs]

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

func (w *Worker) Work(ctx context.Context, job *river.Job[jobs.UploadArgs]) error {
	artifactID := job.Args.ArtifactID
	artifact, err := w.db.Artifact(ctx, artifactID)
	if err != nil {
		return err
	}

	if artifact.Uploaded {
		return nil
	}

	uploader, ok := w.uploaders[artifact.Round.ServerID]
	if !ok {
		return fmt.Errorf("upload_no_handler")
	}

	err = uploader.Upload(ctx, artifact.Artifact)
	if err != nil {
		return err
	}

	return w.db.MarkUploaded(ctx, river.ClientFromContext[*sql.Tx](ctx), artifactID)
}
