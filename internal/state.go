package internal

import (
	"encoding/json"
	"os"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type FileState struct {
	Path         string `json:"path"`
	CreationTime string `json:"creationTime"`
}

type ServerState struct {
	LastRound map[config.ArtifactType]FileState `json:"lastRound"`
}

type State struct {
	filePath string
	Servers  map[string]*Server `json:"servers"`
}

func OpenState(filename string) (*State, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return &State{
				filePath: filename,
				Servers:  make(map[string]*Server),
			}, nil
		}
		return nil, err
	}

	var state State
	err = json.Unmarshal(content, &state)
	if err != nil {
		return nil, err
	}

	state.filePath = filename

	return &state, nil
}

func (s *State) Save() error {
	content, err := json.Marshal(s)
	if err != nil {
		return err
	}

	return os.WriteFile(s.filePath, content, 0644)
}
