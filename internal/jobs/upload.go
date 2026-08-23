package jobs

import (
	"github.com/riverqueue/river"
)

const UploadKind = "upload"

type UploadArgs struct {
	RoundID    uint
	ArtifactID uint
}

func (UploadArgs) Kind() string { return UploadKind }

func (UploadArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}
