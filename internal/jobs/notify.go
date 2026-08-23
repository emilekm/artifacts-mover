package jobs

import (
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const SyncNotificationKind = "notify"

type SyncNotificationArgs struct {
	ServerID string
	RoundID  uint
	DemoName string
}

func (SyncNotificationArgs) Kind() string { return SyncNotificationKind }

func (a SyncNotificationArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{
		Queue: a.ServerID,
		UniqueOpts: river.UniqueOpts{
			ByArgs: true,
			ByState: []rivertype.JobState{
				rivertype.JobStatePending,
				rivertype.JobStateRunning,
				rivertype.JobStateScheduled,
				rivertype.JobStateAvailable,
				rivertype.JobStateRetryable,
			},
		},
	}
}
