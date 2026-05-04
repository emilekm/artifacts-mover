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

type JSONSummary struct {
	MapName      string   `json:"MapName"`
	MapMode      string   `json:"MapMode"`
	MapLayer     int      `json:"MapLayer"`
	Team1Name    string   `json:"Team1Name"`
	Team2Name    string   `json:"Team2Name"`
	Team1Tickets int      `json:"Team1Tickets"`
	Team2Tickets int      `json:"Team2Tickets"`
	StartTime    int64    `json:"StartTime"`
	EndTime      int64    `json:"EndTime"`
	Players      []Player `json:"Players"`
}

type RoundSummary struct {
	JSONSummary

	PRDemoPath string
	RemoteRefs map[config.ArtifactType]RemoteRef
}
