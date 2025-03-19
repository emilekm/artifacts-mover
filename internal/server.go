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
)

type Round struct {
	Files    map[config.ArtifactType]string `yaml:"files"`
	Uploaded bool                           `yaml:"uploaded"`
}

func NewRound() *Round {
	return &Round{
		Files: make(map[config.ArtifactType]string),
	}
}

type Server struct {
	Config       *config.Server
	Rounds       []*Round
	CurrentRound *Round
}

func (s *Server) Upload(round *Round) error {
	var uploader func(*Round) error
	if s.Config.Upload.HTTPS != nil {
		uploader = s.uploadHTTPS
	}

	if s.Config.Upload.SCP != nil {
		uploader = s.uploadSCP
	}

	if uploader == nil {
		return nil
	}

	err := uploader(round)
	if err != nil {
		return err
	}

	round.Uploaded = true
	return nil
}

func (s *Server) uploadHTTPS(round *Round) error {
	conf := s.Config.Upload.HTTPS

	for typ, filename := range round.Files {
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

		uri, err := url.JoinPath(conf.URL, s.Config.Artifacts[typ].SubPath)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", uri, buf)
		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", writer.FormDataContentType())

		if len(conf.Auth.Headers) > 0 {
			for k, v := range conf.Auth.Headers {
				req.Header.Set(k, v)
			}
		}

		if conf.Auth.Basic != nil {
			req.SetBasicAuth(conf.Auth.Basic.Username, conf.Auth.Basic.Password)
		}

		_, err = http.DefaultClient.Do(req)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *Server) uploadSCP(round *Round) error {
	conf := s.Config.Upload.SCP

	privateKey, err := os.ReadFile(conf.PrivateKeyFile)
	if err != nil {
		return err
	}

	scpConf, err := scp.NewSSHConfigFromPrivateKey(conf.Username, privateKey)
	if err != nil {
		return err
	}

	scpClient, err := scp.NewClient(conf.Address, scpConf, &scp.ClientOption{})
	if err != nil {
		return err
	}

	defer scpClient.Close()

	for typ, filename := range round.Files {
		err = scpClient.CopyFileToRemote(
			filename,
			filepath.Join(
				conf.BasePath,
				s.Config.Artifacts[typ].SubPath,
				filepath.Base(filename),
			),
			&scp.FileTransferOption{},
		)
		if err != nil {
			return err
		}
	}

	return nil
}
