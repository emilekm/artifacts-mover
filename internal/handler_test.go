package internal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/require"
	gomock "go.uber.org/mock/gomock"
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
				ctrl := gomock.NewController(t)

				uploader := NewMockUploader(ctrl)
				for _, round := range test.expectedRounds {
					uploader.EXPECT().Upload(round)
				}

				failedDir := t.TempDir()

				handler, err := NewHandler(uploader, nil, test.artifactsConfig, 0, failedDir)
				require.NoError(t, err)

				for _, file := range test.files {
					handler.OnFileCreate(file)
				}
			})
		}
	})

	t.Run("UploadOldFiles", func(t *testing.T) {
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				ctrl := gomock.NewController(t)

				dir := t.TempDir()

				uploader := NewMockUploader(ctrl)
				for _, round := range test.expectedRounds {
					for typ, artifact := range round {
						artifact.Path = filepath.Join(dir, artifact.Path)
						round[typ] = artifact
					}
					uploader.EXPECT().Upload(round)
				}

				for _, file := range test.files {
					require.NoError(t, os.MkdirAll(filepath.Join(dir, filepath.Dir(file)), 0755))
					require.NoError(t, os.WriteFile(filepath.Join(dir, file), []byte("test"), 0644))
				}

				artifactsConfig := test.artifactsConfig
				for typ, loc := range artifactsConfig {
					loc.Location = filepath.Join(dir, loc.Location)
					artifactsConfig[typ] = loc
				}

				failedDir := t.TempDir()

				handler, err := NewHandler(uploader, nil, artifactsConfig, 0, failedDir)
				require.NoError(t, err)

				require.NoError(t, handler.UploadOldFiles())
			})
		}
	})
}

func prepareRound(artifacts map[config.ArtifactType]string) Round {
	round := make(Round)
	for typ, path := range artifacts {
		round[typ] = Artifact{
			Path: path,
			Type: typ,
		}
	}

	return round
}
