package internal

import "context"

type Uploader interface {
	Upload(ctx context.Context, artifact Artifact) (RemoteRef, error)
}
