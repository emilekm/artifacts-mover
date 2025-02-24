package main

import "github.com/emilekm/artifacts-mover/internal/config"

type ArtifactType int

const (
	ArtifactTypeBF2Demo = iota
	ArtifactTypePRDemo
	ArtifactTypeSummary
)

type Round struct {
	BF2DemoFile string `yaml:"bf2DemoFile"`
	PRDemoFile  string `yaml:"prDemoFile"`
	SummaryFile string `yaml:"summaryFile"`
	Uploaded    bool   `yaml:"uploaded"`
}

type Server struct {
	Config       *config.Server
	CurrentRound *Round
	Rounds       []*Round
}
