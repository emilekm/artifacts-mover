package internal

import (
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type UploadedArtifact struct {
	Filename  string
	Type      config.ArtifactType
	RemoteRef string
}

type RoundRecord struct {
	ServerID  string
	RoundKey  string
	Artifacts []UploadedArtifact
	Notified  bool
	CreatedAt time.Time
}

type StateStore interface {
	RecordUpload(serverID, roundKey string, artifact UploadedArtifact) error
	RecordNotified(serverID, roundKey string) error
	QueryUnnotified(serverID string, since time.Time) ([]RoundRecord, error)
	PurgeCompleted(olderThan time.Time) error
	Close() error
}
