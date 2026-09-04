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

// InsertOpts intentionally leaves Queue unset, so uploads run on the shared
// default queue rather than a per-server one: unlike sync notifications, upload
// jobs don't need to be serialized per server — they're independent I/O and are
// fine running concurrently across servers.
func (UploadArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
		},
	}
}
