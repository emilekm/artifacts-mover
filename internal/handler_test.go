package internal

import (
	"testing"

	"github.com/emilekm/artifacts-mover/internal/config"
	gomock "go.uber.org/mock/gomock"
)

func TestHandler(t *testing.T) {
	tests := []struct {
		name          string
		locToTyp      map[string]config.ArtifactType
		uploaderSetup func(*Mockuploader)
		files         []string
	}{
		{
			name: "bf2demo only",
			locToTyp: map[string]config.ArtifactType{
				"bf2demos": config.ArtifactTypeBF2Demo,
			},
			uploaderSetup: func(m *Mockuploader) {
				m.EXPECT().Upload(Round{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
				})
			},
			files: []string{
				"./bf2demos/file1",
				"./bf2demos/file2",
			},
		},
		{
			name: "mixed",
			locToTyp: map[string]config.ArtifactType{
				"bf2demos": config.ArtifactTypeBF2Demo,
				"prdemos":  config.ArtifactTypePRDemo,
				"json":     config.ArtifactTypeSummary,
			},
			uploaderSetup: func(m *Mockuploader) {
				m.EXPECT().Upload(Round{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
					config.ArtifactTypePRDemo:  "prdemos/file1",
					config.ArtifactTypeSummary: "json/file1",
				})
			},
			files: []string{
				"./bf2demos/file1",
				"./prdemos/file1",
				"./json/file1",
				"./bf2demos/file2",
			},
		},
		{
			name: "mixed - missing json",
			locToTyp: map[string]config.ArtifactType{
				"bf2demos": config.ArtifactTypeBF2Demo,
				"prdemos":  config.ArtifactTypePRDemo,
				"json":     config.ArtifactTypeSummary,
			},
			uploaderSetup: func(m *Mockuploader) {
				m.EXPECT().Upload(Round{
					config.ArtifactTypeBF2Demo: "bf2demos/file1",
					config.ArtifactTypePRDemo:  "prdemos/file1",
				})
			},
			files: []string{
				"./bf2demos/file1",
				"./prdemos/file1",
				"./bf2demos/file2",
			},
		},
		{
			name: "non-bf2demo only",
			locToTyp: map[string]config.ArtifactType{
				"prdemos": config.ArtifactTypePRDemo,
				"json":    config.ArtifactTypeSummary,
			},
			uploaderSetup: func(m *Mockuploader) {
				m.EXPECT().Upload(Round{
					config.ArtifactTypePRDemo:  "prdemos/file1",
					config.ArtifactTypeSummary: "json/file1",
				})
			},
			files: []string{
				"./prdemos/file1",
				"./json/file1",
				"./prdemos/file2",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)

			uploader := NewMockuploader(ctrl)
			test.uploaderSetup(uploader)

			handler := NewHandler(uploader, test.locToTyp)

			for _, file := range test.files {
				handler.OnFileCreate(file)
			}
		})
	}
}
