package jobs

type SyncNotificationArgs struct {
	RoundID uint
}

func (SyncNotificationArgs) Kind() string { return "notify" }

// func (SyncNotificationArgs) InsertOpts() river.InsertOpts {
// 	return river.InsertOpts{
// 		UniqueOpts: river.UniqueOpts{
// 			ByArgs: true,
// 			ByState: []rivertype.JobState{
// 				rivertype.JobStatePending,
// 				rivertype.JobStateRunning,
// 				rivertype.JobStateScheduled,
//
// 				rivertype.JobStateAvailable,
// 				rivertype.JobStateRetryable,
// 			},
// 		},
// 	}
// }
