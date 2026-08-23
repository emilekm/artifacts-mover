package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"github.com/emilekm/artifacts-mover/internal/jobs"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DB struct {
	gorm *gorm.DB
}

func NewDB(pool *sql.DB) (*DB, error) {
	db, err := gorm.Open(sqlite.New(sqlite.Config{
		Conn: pool,
	}))
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Round{})
	db.AutoMigrate(&Artifact{})

	return &DB{
		gorm: db,
	}, nil
}

func (db *DB) EnqueueRound(ctx context.Context, riverClient *river.Client[*sql.Tx], serverID string, started time.Time, round types.Round) error {
	tx := db.gorm.Begin()
	defer tx.Rollback()
	dbRound := Round{
		ServerID: serverID,
		Started:  started,
	}

	for _, artifact := range round {
		dbRound.Artifacts = append(dbRound.Artifacts, Artifact{
			Artifact: artifact,
		})
	}

	result := gorm.WithResult()
	err := gorm.G[Round](tx, result).Create(ctx, &dbRound)
	if err != nil && !errors.Is(err, gorm.ErrCheckConstraintViolated) {
		return err
	}

	sqlTx := tx.Statement.Statement.ConnPool.(*sql.Tx)

	for _, artifact := range dbRound.Artifacts {
		_, err = riverClient.InsertTx(ctx, sqlTx, jobs.UploadArgs{
			RoundID:    dbRound.ID,
			ArtifactID: artifact.ID,
		}, nil)
		if err != nil {
			return err
		}
	}

	_, err = riverClient.InsertTx(ctx, sqlTx, jobs.SyncNotificationArgs{
		ServerID:  serverID,
		RoundID:   dbRound.ID,
		Timestamp: started,
	}, nil)
	if err != nil {
		return err
	}

	return tx.Commit().Error
}

func (db *DB) Round(ctx context.Context, roundID uint) (Round, error) {
	return gorm.G[Round](db.gorm).Preload("Artifacts", nil).Where("id = ?", roundID).First(ctx)
}

func (db *DB) Artifact(ctx context.Context, artifactID uint) (Artifact, error) {
	return gorm.G[Artifact](db.gorm).Where("id = ?", artifactID).Preload("Round", nil).First(ctx)
}

func (db *DB) MarkUploaded(ctx context.Context, riverClient *river.Client[*sql.Tx], artifactID uint) error {
	artifact, err := db.Artifact(ctx, artifactID)
	if err != nil {
		return err
	}

	tx := db.gorm.Begin()
	defer tx.Rollback()

	_, err = gorm.G[Artifact](tx).Where("id = ?", artifactID).Update(ctx, "uploaded", true)
	if err != nil {
		return err
	}

	sqlTx := tx.Statement.Statement.ConnPool.(*sql.Tx)
	_, err = riverClient.InsertTx(ctx, sqlTx, jobs.SyncNotificationArgs{
		ServerID:  artifact.Round.ServerID,
		RoundID:   artifact.RoundID,
		Timestamp: artifact.Round.Started,
	}, nil)
	if err != nil {
		return err
	}

	return tx.Commit().Error
}

func (db *DB) UpdateMessageID(ctx context.Context, roundID uint, messageID string) error {
	_, err := gorm.G[Round](db.gorm).Where("id = ?", roundID).Update(ctx, "discord_message_id", messageID)
	return err
}

func (db *DB) CancelWaitingSyncJobs(ctx context.Context, client *river.Client[*sql.Tx], serverID string, roundID uint) error {
	params := river.NewJobListParams().
		Kinds(jobs.SyncNotificationKind).
		States(rivertype.JobStateAvailable).
		Queues(serverID)

	tx := db.gorm.Begin()
	defer tx.Rollback()

	sqlTx := tx.Statement.Statement.ConnPool.(*sql.Tx)

	result, err := client.JobListTx(ctx, sqlTx, params)
	if err != nil {
		return err
	}

	for _, job := range result.Jobs {
		var args jobs.SyncNotificationArgs
		err := json.Unmarshal(job.EncodedArgs, &args)
		if err != nil {
			return err
		}

		if args.RoundID == roundID {
			_, err = client.JobCancelTx(ctx, sqlTx, job.ID)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit().Error
}
