package ingest

import "context"

type Source interface {
	Run(ctx context.Context) error
}
