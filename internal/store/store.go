package store

import (
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
)

type Store interface {
	EnqueueRound(r types.Round) error

	PendingUploads() ([]types.Round, error) // uploaded==false AND next_attempt_at<=now
	MarkUploaded(serverID, roundID string, t types.ArtifactType) error

	PendingNotifications() ([]types.Round, error) // all uploaded AND notified==false AND next_attempt_at<=now
	MarkNotified(serverID, roundID string) error

	Backoff(serverID, roundID string, err error) error // attempts++, next_attempt_at, dead-letter at max
	PurgeCompleted(olderThan time.Time) error
}
