package ingest

import (
	"context"
	"log/slog"
	"testing"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name            string
		artifactsConfig config.ArtifactsConfig
		expectedRounds  []types.Round
		files           []string
	}{
		{
			name: "bf2demo only",
			artifactsConfig: config.ArtifactsConfig{
				types.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
			},
			expectedRounds: []types.Round{
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypeBF2Demo: "bf2demos/file1",
				}),
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypeBF2Demo: "bf2demos/file2",
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
				types.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
				types.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				types.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []types.Round{
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypeBF2Demo: "bf2demos/file1",
					types.ArtifactTypePRDemo:  "prdemos/file1",
					types.ArtifactTypeSummary: "json/file1",
				}),
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypeBF2Demo: "bf2demos/file2",
					types.ArtifactTypePRDemo:  "prdemos/file2",
					types.ArtifactTypeSummary: "json/file2",
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
				types.ArtifactTypeBF2Demo: config.Location{Location: "bf2demos"},
				types.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				types.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []types.Round{
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypeBF2Demo: "bf2demos/file1",
					types.ArtifactTypePRDemo:  "prdemos/file1",
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
				types.ArtifactTypePRDemo:  config.Location{Location: "prdemos"},
				types.ArtifactTypeSummary: config.Location{Location: "json"},
			},
			expectedRounds: []types.Round{
				prepareRound(map[types.ArtifactType]string{
					types.ArtifactTypePRDemo:  "prdemos/file1",
					types.ArtifactTypeSummary: "json/file1",
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
				storeMock := store.NewMockStore()

				var enqueued []types.Round
				storeMock.On("EnqueueRound", mock.Anything).
					Run(func(args mock.Arguments) {
						enqueued = append(enqueued, args.Get(0).(types.Round))
					}).
					Return(nil)

				handler := NewHandler(
					testCtx(t),
					testLogger(t),
					storeMock,
					test.artifactsConfig,
					0,
					"serverID",
				)
				defer handler.Close()

				for _, file := range test.files {
					handler.OnFileCreate(file)
				}

				require.Len(t, enqueued, len(test.expectedRounds))
				for i, expected := range test.expectedRounds {
					assert.Equal(t, expected.ServerID, enqueued[i].ServerID)
					assert.Equal(t, expected.Artifacts, enqueued[i].Artifacts)
				}
			})
		}
	})

}

func prepareRound(artifacts map[types.ArtifactType]string) types.Round {
	round := types.NewRound("serverID")
	for typ, path := range artifacts {
		round.Artifacts[typ] = types.NewArtifact(path, typ)
	}
	return *round
}

func testLogger(t *testing.T) *slog.Logger {
	t.Helper()
	opts := &slog.HandlerOptions{AddSource: true}
	log := slog.New(slog.NewTextHandler(t.Output(), opts))
	return log
}

func testCtx(t *testing.T) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}
