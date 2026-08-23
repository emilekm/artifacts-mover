package jobs

type UploadArgs struct {
	RoundID    uint
	ArtifactID uint
}

func (UploadArgs) Kind() string { return "upload" }
