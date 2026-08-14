package types

import "time"

type Round struct {
	ServerID  string
	RoundID   string
	Artifacts map[ArtifactType]Artifact
	Uploaded  bool
	Published bool

	// FirstFailedAt is when publishing this round started failing; zero once it
	// succeeds. It bounds how long a round may hold up the ones behind it.
	FirstFailedAt time.Time
}

func NewRound(serverID string) *Round {
	return &Round{
		ServerID:  serverID,
		Artifacts: make(map[ArtifactType]Artifact),
	}
}
