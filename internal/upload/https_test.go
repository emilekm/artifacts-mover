package upload

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testUploadPaths = map[types.ArtifactType]string{
	types.ArtifactTypeBF2Demo: "upload/bf2demo",
}

// The happy path asserts on the whole request, so it does not share a shape
// with the failure cases below.
func TestHTTPSUploaderUpload(t *testing.T) {
	artifactPath := writeTempArtifact(t, "demo.bf2demo", "demo contents")

	var (
		gotPath        string
		gotMethod      string
		gotFilename    string
		gotContents    string
		gotHeader      string
		gotUser        string
		gotPassword    string
		gotBasicAuthOK bool
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		gotHeader = r.Header.Get("X-Api-Key")
		gotUser, gotPassword, gotBasicAuthOK = r.BasicAuth()

		file, header, err := r.FormFile("artifact")
		require.NoError(t, err)
		defer file.Close()

		gotFilename = header.Filename
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		gotContents = string(contents)
	}))
	defer server.Close()

	uploader := NewHTTPSUploader(config.HTTPSConfig{
		URL: server.URL,
		Auth: config.HTTPSAuth{
			Headers: map[string]string{"X-Api-Key": "secret"},
			Basic:   &config.BasicAuth{Username: "user", Password: "pass"},
		},
	}, testUploadPaths)

	err := uploader.Upload(t.Context(), types.NewArtifact(artifactPath, types.ArtifactTypeBF2Demo))
	require.NoError(t, err)

	assert.Equal(t, http.MethodPost, gotMethod)
	assert.Equal(t, "/upload/bf2demo", gotPath)
	assert.Equal(t, "demo.bf2demo", gotFilename)
	assert.Equal(t, "demo contents", gotContents)
	assert.Equal(t, "secret", gotHeader)
	assert.True(t, gotBasicAuthOK)
	assert.Equal(t, "user", gotUser)
	assert.Equal(t, "pass", gotPassword)
}

func TestHTTPSUploaderUploadErrors(t *testing.T) {
	tests := []struct {
		name            string
		artifactType    types.ArtifactType
		skipFile        bool
		status          int
		wantErrContains string
		wantErrIs       error
		wantRequest     bool
	}{
		{
			name:            "error status",
			artifactType:    types.ArtifactTypeBF2Demo,
			status:          http.StatusInternalServerError,
			wantErrContains: "500",
			wantRequest:     true,
		},
		{
			name:            "type has no upload path",
			artifactType:    types.ArtifactTypePRDemo,
			wantErrContains: "no upload path",
		},
		{
			name:         "file is missing",
			artifactType: types.ArtifactTypeBF2Demo,
			skipFile:     true,
			wantErrIs:    os.ErrNotExist,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			artifactPath := filepath.Join(t.TempDir(), "demo.bf2demo")
			if !test.skipFile {
				artifactPath = writeTempArtifact(t, "demo.bf2demo", "demo contents")
			}

			var gotRequest bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotRequest = true
				if test.status != 0 {
					http.Error(w, "nope", test.status)
				}
			}))
			defer server.Close()

			uploader := NewHTTPSUploader(config.HTTPSConfig{URL: server.URL}, testUploadPaths)

			err := uploader.Upload(t.Context(), types.NewArtifact(artifactPath, test.artifactType))

			require.Error(t, err)
			if test.wantErrIs != nil {
				assert.ErrorIs(t, err, test.wantErrIs)
			}
			if test.wantErrContains != "" {
				assert.Contains(t, err.Error(), test.wantErrContains)
			}
			assert.Equal(t, test.wantRequest, gotRequest)
		})
	}
}

func writeTempArtifact(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}
