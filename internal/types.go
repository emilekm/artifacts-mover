package internal

import "github.com/emilekm/artifacts-mover/internal/config"

type Artifact struct {
	Path string
	Type config.ArtifactType
}
