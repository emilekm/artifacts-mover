package internal

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"golang.org/x/sync/errgroup"
)

type uploader interface {
	Upload(Round) error
}

type Uploader struct {
	artifactsConfig  config.ArtifactsConfig
	queue            *Queue
	failedUploadPath string

	uploaders []uploader
}

func NewUploader(
	queue *Queue,
	conf config.UploadConfig,
	artifactsConfing config.ArtifactsConfig,
	failedUploadPath string,
) (*Uploader, error) {
	uploaders := make([]uploader, 0)
	if conf.SCP != nil {
		uploader, err := newSCPUploader(*conf.SCP, artifactsConfing)
		if err != nil {
			return nil, err
		}
		uploaders = append(uploaders, uploader)
	}

	if conf.HTTPS != nil {
		uploaders = append(uploaders, newHTTPSUploader(*conf.HTTPS, artifactsConfing))
	}

	return &Uploader{
		artifactsConfig:  artifactsConfing,
		queue:            queue,
		failedUploadPath: failedUploadPath,
		uploaders:        uploaders,
	}, nil
}

func (u *Uploader) Upload(round Round) {
	wg := errgroup.Group{}
	for _, up := range u.uploaders {
		errCh := u.queue.Add(func() error {
			return up.Upload(round)
		})
		wg.Go(func() error {
			err, _ := <-errCh
			return err
		})
	}

	go func() {
		err := wg.Wait()
		if err != nil {
			slog.Error("failed to upload round", "err", err)
			u.backupFailures(round)
			return
		}

		u.afterUpload(round)
	}()
}

func (u *Uploader) backupFailures(round Round) {
	for typ, path := range round {
		failedDir := filepath.Join(u.failedUploadPath, typ.String())
		if err := os.Mkdir(failedDir, 0755); err != nil {
			slog.Error("failed to create directory", "path", failedDir, "err", err)
			return
		}

		newPath := filepath.Join(failedDir, filepath.Base(path))
		if err := os.Rename(path, newPath); err != nil {
			slog.Error("failed to move file", "src", path, "dst", newPath, "err", err)
		}
	}
}

func (u *Uploader) afterUpload(round Round) {
	for typ, path := range round {
		artifactConfig := u.artifactsConfig[typ]
		if artifactConfig.MovePath != nil {
			newPath := filepath.Join(*artifactConfig.MovePath, filepath.Base(path))
			err := os.Rename(path, newPath)
			if err != nil {
				slog.Error("failed to move file", "path", path, "err", err)
			}
		} else {
			if err := os.Remove(path); err != nil {
				slog.Error("failed to remove file", "path", path, "err", err)
			}
		}
	}
}
