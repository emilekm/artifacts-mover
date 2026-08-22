package db

import (
	"database/sql"
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type DB struct {
	*gorm.DB
}

func NewDB(pool *sql.DB) (*DB, error) {
	db, err := gorm.Open(sqlite.Open(""))
	if err != nil {
		return nil, err
	}
	db.AutoMigrate(&Round{})
	db.AutoMigrate(&Artifact{})

	return &DB{
		DB: db,
	}, nil
}

type Store interface {
	EnqueueRound(r types.Round) error

	// PendingUploads() ([]types.Round, error) // uploaded==false
	MarkUploaded(serverID, roundID string, t types.ArtifactType) error

	// UnpublishedRounds returns every published==false round grouped by server,
	// oldest first within each server. Publication order depends on that
	// ordering, so it is part of the contract, not an artifact of the storage.
	// UnpublishedRounds() (map[string][]types.Round, error)
	MarkPublished(serverID, roundID string) error
	RecordFailure(serverID, roundID string) error // stamps FirstFailedAt on the first failure

	PurgeCompleted(olderThan time.Time) error
}
