package internal

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"golang.org/x/sync/errgroup"
)

//go:generate go run go.uber.org/mock/mockgen -source=./uploader.go -destination=./uploader_mock.go -package=internal uploader

type uploader interface {
	Upload(Round) error
}

type multiUploader struct {
	artifactsConfig  config.ArtifactsConfig
	queue            *Queue
	failedUploadPath string

	uploaders []uploader
}

func NewMultiUploader(
	queue *Queue,
	conf config.UploadConfig,
	artifactsConfing config.ArtifactsConfig,
	failedUploadPath string,
) (*multiUploader, error) {
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

	return &multiUploader{
		artifactsConfig:  artifactsConfing,
		queue:            queue,
		failedUploadPath: failedUploadPath,
		uploaders:        uploaders,
	}, nil
}

func (u *multiUploader) Upload(round Round) error {
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

	return nil
}

func (u *multiUploader) backupFailures(round Round) {
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

func (u *multiUploader) afterUpload(round Round) {
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
