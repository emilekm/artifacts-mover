package internal

import (
	"sync"
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name            string
		artifactsConfig config.ArtifactsConfig
		expectedRounds  []Round
		files           []string
	}{
		{
			name: "bf2demo only",
			artifactsConfig: config.ArtifactsConfig{
				config.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
			},
			expectedRounds: []Round{
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
				}),
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypeBF2Demo: "bf2demos/file2",
				}),
			},
			files: []string{
				"./bf2demos/file1",
				"./bf2demos/file2",
				"./bf2demos/file3",
			},
		},
		{
			name: "mixed",
			artifactsConfig: config.ArtifactsConfig{
				config.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
				config.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				config.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []Round{
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
					config.ArtifactTypePRDemo:  "prdemos/file1",
					config.ArtifactTypeSummary: "json/file1",
				}),
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypeBF2Demo: "bf2demos/file2",
					config.ArtifactTypePRDemo:  "prdemos/file2",
					config.ArtifactTypeSummary: "json/file2",
				}),
			},
			files: []string{
				"./bf2demos/file1",
				"./prdemos/file1",
				"./json/file1",
				"./bf2demos/file2",
				"./prdemos/file2",
				"./json/file2",
				"./bf2demos/file3",
			},
		},
		{
			name: "mixed - missing json",
			artifactsConfig: config.ArtifactsConfig{
				config.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
				config.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				config.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []Round{
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
					config.ArtifactTypePRDemo:  "prdemos/file1",
				}),
			},
			files: []string{
				"./bf2demos/file1",
				"./prdemos/file1",
				"./bf2demos/file2",
			},
		},
		{
			name: "non-bf2demo only",
			artifactsConfig: config.ArtifactsConfig{
				config.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				config.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []Round{
				prepareRound(map[config.ArtifactType]string{
					config.ArtifactTypePRDemo:  "prdemos/file1",
					config.ArtifactTypeSummary: "json/file1",
				}),
			},
			files: []string{
				"./prdemos/file1",
				"./json/file1",
				"./prdemos/file2",
			},
		},
	}

	t.Run("OnFileCreate", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				rc := newRoundCapture(len(test.expectedRounds))

				handler, err := NewHandler(rc.process, test.artifactsConfig, 0)
				require.NoError(t, err)

				for _, file := range test.files {
					handler.OnFileCreate(file)
				}

				rounds := rc.wait(t)
				assert.ElementsMatch(t, test.expectedRounds, rounds)
			})
		}
	})

}

// roundCapture collects rounds emitted by Handler via the process callback.
type roundCapture struct {
	mu     sync.Mutex
	wg     sync.WaitGroup
	rounds []Round
}

func newRoundCapture(n int) *roundCapture {
	rc := &roundCapture{}
	rc.wg.Add(n)
	return rc
}

func (rc *roundCapture) process(r Round) {
	rc.mu.Lock()
	rc.rounds = append(rc.rounds, r)
	rc.mu.Unlock()
	rc.wg.Done()
}

func (rc *roundCapture) wait(t *testing.T) []Round {
	t.Helper()
	done := make(chan struct{})
	go func() { rc.wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for rounds")
	}
	rc.mu.Lock()
	defer rc.mu.Unlock()
	return rc.rounds
}

func prepareRound(artifacts map[config.ArtifactType]string) Round {
	round := make(Round)
	for typ, path := range artifacts {
		round[typ] = Artifact{Path: path, Type: typ}
	}
	return round
}

