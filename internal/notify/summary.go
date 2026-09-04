package notify

import (
	"io"
)

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

type Ref struct {
	Enabled bool
	URL     string
}

type RemoteRefs struct {
	BF2Demo       Ref
	PRDemo        Ref
	TrackerViewer Ref
}

type Summary struct {
	JSONSummary

	PRDemoPath string
	PRDemoFile io.Reader
	Image      io.Reader
	RemoteRefs RemoteRefs
}
