package internal

import (
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
)

type httpsUploader struct {
	artifactsConfig config.ArtifactsConfig
	conf            config.HTTPSConfig
}

func NewHTTPSUploader(
	conf config.HTTPSConfig,
	artifactsConf config.ArtifactsConfig,
) *httpsUploader {
	return &httpsUploader{
		artifactsConfig: artifactsConf,
		conf:            conf,
	}
}

func (u *httpsUploader) Upload(round Round) error {
	log := slog.With("op", "httpsUploader.Upload")

	for typ, artifact := range round {
		err := u.uploadFile(typ, artifact.Path)
		if err != nil {
			return err
		}
		log.Debug("uploaded file via HTTPS", "path", artifact.Path)
	}

	return nil
}

func (u *httpsUploader) uploadFile(typ config.ArtifactType, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	pr, pw := io.Pipe()

	mw := multipart.NewWriter(pw)

	errCh := make(chan error, 1)
	go func() {
		defer pw.Close()
		defer mw.Close()

		part, err := mw.CreateFormFile("artifact", filepath.Base(filename))
		if err != nil {
			errCh <- err
			return
		}

		if _, err := io.Copy(part, file); err != nil {
			errCh <- err
			return
		}

		errCh <- nil
	}()

	uri, err := url.JoinPath(u.conf.URL, u.artifactsConfig[typ].UploadPath)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uri, pr)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", mw.FormDataContentType())

	if len(u.conf.Auth.Headers) > 0 {
		for k, v := range u.conf.Auth.Headers {
			req.Header.Set(k, v)
		}
	}

	if u.conf.Auth.Basic != nil {
		req.SetBasicAuth(u.conf.Auth.Basic.Username, u.conf.Auth.Basic.Password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if copyErr := <-errCh; copyErr != nil {
		return copyErr
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("upload failed with status: %s", resp.Status)
	}

	return nil
}
