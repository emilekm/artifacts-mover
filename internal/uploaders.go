package internal

import (
	"bytes"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/povsister/scp"
)

type Uploader interface {
	Upload(Round)
}

type SCPUploader struct {
	queue  *Queue
	client *scp.Client

	basePath string
	subPaths map[config.ArtifactType]string
}

func NewSCPUploader(
	queue *Queue,
	conf config.SCPConfig,
	artifactsConf config.ArtifactsConfig,
) (*SCPUploader, error) {
	privKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return nil, err
	}

	scpConf, err := scp.NewSSHConfigFromPrivateKey(conf.Username, privKey)
	if err != nil {
		return nil, err
	}

	scpClient, err := scp.NewClient(conf.Address, scpConf, &scp.ClientOption{})
	if err != nil {
		return nil, err
	}

	subPaths := make(map[config.ArtifactType]string)
	for typ, loc := range artifactsConf {
		subPaths[typ] = loc.SubPath
	}

	return &SCPUploader{
		queue:    queue,
		client:   scpClient,
		basePath: conf.BasePath,
		subPaths: subPaths,
	}, nil
}

func (u *SCPUploader) Close() {
	u.client.Close()
}

func (u *SCPUploader) Upload(round Round) {
	u.queue.Add(func() {
		u.upload(round)
	})
}

func (u *SCPUploader) upload(round Round) {
	for typ, path := range round {
		err := u.client.CopyFileToRemote(
			path,
			filepath.Join(
				u.basePath,
				u.subPaths[typ],
				filepath.Base(path),
			),
			&scp.FileTransferOption{},
		)
		if err != nil {
			slog.Error("failed to upload file", "path", path, "err", err)
		}
	}
}

type HTTPSUploader struct {
	queue    *Queue
	conf     config.HTTPSConfig
	subPaths map[config.ArtifactType]string
}

func NewHTTPSUploader(
	queue *Queue,
	conf config.HTTPSConfig,
	artifactsConf config.ArtifactsConfig,
) *HTTPSUploader {
	subPaths := make(map[config.ArtifactType]string)
	for typ, loc := range artifactsConf {
		subPaths[typ] = loc.SubPath
	}

	return &HTTPSUploader{
		queue:    queue,
		conf:     conf,
		subPaths: subPaths,
	}
}

func (u *HTTPSUploader) Upload(round Round) {
	u.queue.Add(func() {
		u.upload(round)
	})
}

func (u *HTTPSUploader) upload(round Round) {
	for typ, filename := range round {
		err := u.uploadFile(typ, filename)
		if err != nil {
			slog.Error("failed to upload file", "path", filename, "err", err)
		}
	}
}

func (u *HTTPSUploader) uploadFile(typ config.ArtifactType, filename string) error {
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

	uri, err := url.JoinPath(u.conf.URL, u.subPaths[typ])
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
