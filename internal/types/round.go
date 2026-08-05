package types

type Round struct {
	ServerID  string
	RoundID   string
	Artifacts map[ArtifactType]Artifact
	Uploaded  bool
	Notified  bool
}

func NewRound(serverID string) *Round {
	return &Round{
		ServerID:  serverID,
		Artifacts: make(map[ArtifactType]Artifact),
	}
}
