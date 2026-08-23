package jobs

const UploadKind = "upload"

type UploadArgs struct {
	RoundID    uint
	ArtifactID uint
}

func (UploadArgs) Kind() string { return UploadKind }
