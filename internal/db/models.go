package db

import (
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	"gorm.io/gorm"
)

type Artifact struct {
	gorm.Model
	types.Artifact
	RoundID uint
	Round   Round
}

type Round struct {
	gorm.Model
	ServerID         string
	Started          time.Time
	Artifacts        []Artifact
	ArtifactsByType  types.Round `gorm:"-"`
	DiscordMessageID *string
}

func (r *Round) AfterFind(tx *gorm.DB) error {
	r.ArtifactsByType = make(types.Round, len(r.Artifacts))
	for _, a := range r.Artifacts {
		r.ArtifactsByType[a.Type] = a.Artifact
	}
	return nil
}
