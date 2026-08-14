package store

import (
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
)

type Store interface {
	EnqueueRound(r types.Round) error

	PendingUploads() ([]types.Round, error) // uploaded==false
	MarkUploaded(serverID, roundID string, t types.ArtifactType) error

	// UnpublishedRounds returns every published==false round grouped by server,
	// oldest first within each server. Publication order depends on that
	// ordering, so it is part of the contract, not an artifact of the storage.
	UnpublishedRounds() (map[string][]types.Round, error)
	MarkPublished(serverID, roundID string) error
	RecordFailure(serverID, roundID string) error // stamps FirstFailedAt on the first failure

	PurgeCompleted(olderThan time.Time) error
}
