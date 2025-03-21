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
	"golang.org/x/crypto/ssh"
)

type Uploader interface {
	Upload(Round)
}

func NewUploader(
	queue *Queue,
	conf config.UploadConfig,
	artifactsConf config.ArtifactsConfig,
) (Uploader, error) {
	if conf.SCP != nil {
		return NewSCPUploader(queue, *conf.SCP, artifactsConf)
	}

	if conf.HTTPS != nil {
		return NewHTTPSUploader(queue, *conf.HTTPS, artifactsConf), nil
	}

	return nil, nil
}

type uploader struct {
	queue           *Queue
	artifactsConfig config.ArtifactsConfig
}

func (u *uploader) afterUpload(round Round) {
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

type SCPUploader struct {
	uploader

	basePath string
	address  string
	scpConf  *ssh.ClientConfig
}

func NewSCPUploader(
	queue *Queue,
	conf config.SCPConfig,
	artifactsConfig config.ArtifactsConfig,
) (*SCPUploader, error) {
	privKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return nil, err
	}

	scpConf, err := scp.NewSSHConfigFromPrivateKey(conf.Username, privKey)
	if err != nil {
		return nil, err
	}

	u := &SCPUploader{
		uploader: uploader{
			queue:           queue,
			artifactsConfig: artifactsConfig,
		},
		basePath: conf.BasePath,
		scpConf:  scpConf,
	}

	_, err = u.client()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (u *SCPUploader) Upload(round Round) {
	u.queue.Add(func() {
		u.upload(round)
	})
}

func (u *SCPUploader) upload(round Round) {
	client, err := u.client()
	if err != nil {
		slog.Error("failed to create scp client", "err", err)
		return
	}

	for typ, path := range round {
		err := client.CopyFileToRemote(
			path,
			u.fullUploadPath(typ, path),
			&scp.FileTransferOption{},
		)
		if err != nil {
			slog.Error("failed to upload file", "path", path, "err", err)
		}
	}

	u.afterUpload(round)
}

func (u *SCPUploader) fullUploadPath(typ config.ArtifactType, path string) string {
	filename := filepath.Base(path)
	return filepath.Join(u.basePath, u.artifactsConfig[typ].UploadPath, filename)
}

func (u *SCPUploader) client() (*scp.Client, error) {
	return scp.NewClient(u.address, u.scpConf, &scp.ClientOption{})
}

type HTTPSUploader struct {
	uploader
	conf config.HTTPSConfig
}

func NewHTTPSUploader(
	queue *Queue,
	conf config.HTTPSConfig,
	artifactsConf config.ArtifactsConfig,
) *HTTPSUploader {
	return &HTTPSUploader{
		uploader: uploader{
			queue:           queue,
			artifactsConfig: artifactsConf,
		},
		conf: conf,
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

	u.afterUpload(round)
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
