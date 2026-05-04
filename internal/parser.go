package internal

import (
	"encoding/json"
	"os"
)

func ParseSummary(path string) (*RoundSummary, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var summary JSONSummary

	if err := json.Unmarshal(data, &summary); err != nil {
		return nil, err
	}

	return &RoundSummary{
		JSONSummary: summary,
	}, nil
}
