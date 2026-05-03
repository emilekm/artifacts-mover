package internal

import (
	"encoding/json"
	"os"
)

func ParseSummary(path string) (RoundSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return RoundSummary{}, err
	}

	var raw struct {
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

	if err := json.Unmarshal(data, &raw); err != nil {
		return RoundSummary{}, err
	}

	return RoundSummary{
		MapName:      raw.MapName,
		MapMode:      raw.MapMode,
		MapLayer:     raw.MapLayer,
		Team1Name:    raw.Team1Name,
		Team2Name:    raw.Team2Name,
		Team1Tickets: raw.Team1Tickets,
		Team2Tickets: raw.Team2Tickets,
		StartTime:    raw.StartTime,
		EndTime:      raw.EndTime,
		Players:      raw.Players,
	}, nil
}
