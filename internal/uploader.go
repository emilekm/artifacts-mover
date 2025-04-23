package internal

import (
	"fmt"
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
	log := slog.With("op", "multiUploader.Upload")

	wg := errgroup.Group{}
	for _, up := range u.uploaders {
		log.Debug(fmt.Sprintf("Adding uploader %T to queue", up))
		errCh := u.queue.Add(func() error {
			return up.Upload(round)
		})
		wg.Go(func() error {
			err, closed := <-errCh
			if closed {
				log.Debug("Upload successful", "uploader", fmt.Sprintf("%T", up))
				return nil
			}
			if err != nil {
				log.Error(fmt.Sprintf("Received error from %T uploader", up), "err", err)
				return err
			}
			return nil
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
	log := slog.With("op", "multiUploader.backupFailures")

	for typ, artifacts := range round {
		failedDir := filepath.Join(u.failedUploadPath, typ.String())
		if err := os.Mkdir(failedDir, 0755); err != nil {
			log.Error("failed to create directory", "path", failedDir, "err", err)
			return
		}

		newPath := filepath.Join(failedDir, filepath.Base(artifacts.Path))
		if err := os.Rename(artifacts.Path, newPath); err != nil {
			log.Error("failed to move file", "src", artifacts, "dst", newPath, "err", err)
		}
	}
}

func (u *multiUploader) afterUpload(round Round) {
	log := slog.With("op", "multiUploader.afterUpload")

	for typ, artifact := range round {
		artifactConfig := u.artifactsConfig[typ]
		if artifactConfig.MovePath != nil {
			newPath := filepath.Join(*artifactConfig.MovePath, filepath.Base(artifact.Path))
			err := os.Rename(artifact.Path, newPath)
			if err != nil {
				log.Error("failed to move file", "path", artifact, "err", err)
			}
		} else {
			if err := os.Remove(artifact.Path); err != nil {
				log.Error("failed to remove file", "path", artifact, "err", err)
			}
		}
	}
}
