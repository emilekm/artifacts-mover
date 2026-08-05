package types

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"
)

//go:generate go tool golang.org/x/tools/cmd/stringer -type=ArtifactType -linecomment -output=artifact_string.go

const filenameLayout = "2006_01_02_15_04_05"

var tsRe = regexp.MustCompile(`\d{4}_\d{2}_\d{2}_\d{2}_\d{2}_\d{2}`)

type ArtifactType int

const (
	ArtifactTypeBF2Demo ArtifactType = iota // bf2demo
	ArtifactTypePRDemo                      // prdemo
	ArtifactTypeSummary                     // summary
)

func (i ArtifactType) MarshalText() ([]byte, error) {
	return []byte(i.String()), nil
}

func (i *ArtifactType) UnmarshalText(text []byte) error {
	switch string(text) {
	case ArtifactTypeBF2Demo.String():
		*i = ArtifactTypeBF2Demo
	case ArtifactTypePRDemo.String():
		*i = ArtifactTypePRDemo
	case ArtifactTypeSummary.String():
		*i = ArtifactTypeSummary
	default:
		return fmt.Errorf("unknown value %s", string(text))
	}

	return nil
}

type Artifact struct {
	Type      ArtifactType
	Path      string
	Timestamp *time.Time
	Uploaded  bool
}

func NewArtifact(path string, typ ArtifactType) Artifact {
	a := Artifact{
		Path: path,
		Type: typ,
	}
	if timestamp, found := artifactTimestamp(path); found {
		a.Timestamp = &timestamp
	}
	return a
}

func artifactTimestamp(path string) (time.Time, bool) {
	m := tsRe.FindString(filepath.Base(path))
	if m == "" {
		return time.Time{}, false
	}
	t, err := time.Parse(filenameLayout, m)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}
