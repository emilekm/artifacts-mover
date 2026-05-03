package internal

import "github.com/emilekm/artifacts-mover/internal/config"

type RemoteRef = string

type Artifact struct {
	Path       string
	Type       config.ArtifactType
	UploadPath string
}
