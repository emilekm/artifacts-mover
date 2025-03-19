package config

import "fmt"

//go:generate go run golang.org/x/tools/cmd/stringer -type=ArtifactType -linecomment -output=artifacttype_string.go

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
