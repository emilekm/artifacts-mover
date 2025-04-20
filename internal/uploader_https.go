package internal

import (
	"bytes"
	"io"
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

func newHTTPSUploader(
	conf config.HTTPSConfig,
	artifactsConf config.ArtifactsConfig,
) *httpsUploader {
	return &httpsUploader{
		artifactsConfig: artifactsConf,
		conf:            conf,
	}
}

func (u *httpsUploader) Upload(round Round) error {
	for typ, artifact := range round {
		err := u.uploadFile(typ, artifact.Path)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *httpsUploader) uploadFile(typ config.ArtifactType, filename string) error {
	buf := &bytes.Buffer{}

	writer := multipart.NewWriter(buf)
	defer writer.Close()

	part, err := writer.CreateFormFile("artifact", filepath.Base(filename))
	if err != nil {
		return err
	}

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err = io.Copy(part, file); err != nil {
		return err
	}

	uri, err := url.JoinPath(u.conf.URL, u.artifactsConfig[typ].UploadPath)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", uri, buf)
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	if len(u.conf.Auth.Headers) > 0 {
		for k, v := range u.conf.Auth.Headers {
			req.Header.Set(k, v)
		}
	}

	if u.conf.Auth.Basic != nil {
		req.SetBasicAuth(u.conf.Auth.Basic.Username, u.conf.Auth.Basic.Password)
	}

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	return nil
}
