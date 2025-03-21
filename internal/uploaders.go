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
	"github.com/povsister/scp"
	"golang.org/x/crypto/ssh"
)

type scpUploader struct {
	artifactsConfig config.ArtifactsConfig

	basePath string
	address  string
	scpConf  *ssh.ClientConfig
}

func newSCPUploader(
	conf config.SCPConfig,
	artifactsConfig config.ArtifactsConfig,
) (*scpUploader, error) {
	privKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return nil, err
	}

	scpConf, err := scp.NewSSHConfigFromPrivateKey(conf.Username, privKey)
	if err != nil {
		return nil, err
	}

	u := &scpUploader{
		artifactsConfig: artifactsConfig,
		basePath:        conf.BasePath,
		scpConf:         scpConf,
	}

	_, err = u.client()
	if err != nil {
		return nil, err
	}

	return u, nil
}

func (u *scpUploader) Upload(round Round) error {
	client, err := u.client()
	if err != nil {
		return err
	}

	for typ, path := range round {
		err := client.CopyFileToRemote(
			path,
			u.fullUploadPath(typ, path),
			&scp.FileTransferOption{},
		)
		if err != nil {
			return err
		}
	}

	return nil
}

func (u *scpUploader) fullUploadPath(typ config.ArtifactType, path string) string {
	filename := filepath.Base(path)
	return filepath.Join(u.basePath, u.artifactsConfig[typ].UploadPath, filename)
}

func (u *scpUploader) client() (*scp.Client, error) {
	return scp.NewClient(u.address, u.scpConf, &scp.ClientOption{})
}

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
	for typ, filename := range round {
		err := u.uploadFile(typ, filename)
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
