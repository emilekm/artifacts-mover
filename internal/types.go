package internal

import "github.com/emilekm/artifacts-mover/internal/config"

type RemoteRef = string

type Artifact struct {
	Path       string
	Type       config.ArtifactType
	UploadPath string
}

type Player struct {
	Name  string
	Score int
}

type RoundSummary struct {
	MapName      string
	MapMode      string
	MapLayer     int
	Team1Name    string
	Team2Name    string
	Team1Tickets int
	Team2Tickets int
	StartTime    int64
	EndTime      int64
	Players      []Player

	PRDemoPath string
	RemoteRefs map[config.ArtifactType]RemoteRef
}
