package upload

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/types"
)

type httpsUploader struct {
	conf        config.HTTPSConfig
	uploadPaths map[types.ArtifactType]string
}

func NewHTTPSUploader(conf config.HTTPSConfig, uploadPaths map[types.ArtifactType]string) *httpsUploader {
	return &httpsUploader{conf: conf, uploadPaths: uploadPaths}
}

func (u *httpsUploader) Upload(ctx context.Context, artifact types.Artifact) error {
	uploadPath, ok := u.uploadPaths[artifact.Type]
	if !ok {
		return fmt.Errorf("https_uploader: no upload path for artifact of type %q", artifact.Type)
	}

	postURL, err := url.JoinPath(u.conf.URL, uploadPath)
	if err != nil {
		return err
	}

	return u.uploadFile(ctx, postURL, artifact.Path)
}

func (u *httpsUploader) uploadFile(ctx context.Context, postURL, filename string) error {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, postURL, pr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	for k, v := range u.conf.Auth.Headers {
		req.Header.Set(k, v)
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
