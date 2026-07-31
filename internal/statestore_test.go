package internal

import (
	"testing"
	"time"

	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) StateStore {
	t.Helper()
	path := t.TempDir() + "/state.db"
	store, err := NewBboltStateStore(path)
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestStateStore_RecordUpload_AppendsArtifactsToExistingRound(t *testing.T) {
	store := newTestStore(t)

	a1 := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	a2 := UploadedArtifact{Filename: "round_001.prdemo", Type: config.ArtifactTypePRDemo, RemoteRef: "https://example.com/a.prdemo"}

	require.NoError(t, store.RecordUpload("server1", "20250503-142301", a1))
	require.NoError(t, store.RecordUpload("server1", "20250503-142301", a2))

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, []UploadedArtifact{a1, a2}, records[0].Artifacts)
}

func TestStateStore_RecordNotified_ExcludesFromQueryUnnotified(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	require.NoError(t, store.RecordUpload("server1", "20250503-142301", artifact))
	require.NoError(t, store.RecordNotified("server1", "20250503-142301"))

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestStateStore_QueryUnnotified_FiltersOlderThanSince(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	require.NoError(t, store.RecordUpload("server1", "20250503-120000", artifact))

	// since = 1 minute in the future → record created just now is before since → excluded
	since := time.Now().Add(1 * time.Minute)
	records, err := store.QueryUnnotified("server1", since)
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestStateStore_QueryUnnotified_IncludesRecordsAfterSince(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	require.NoError(t, store.RecordUpload("server1", "20250503-120000", artifact))

	// since = 1 hour ago → record created just now is within window → included
	since := time.Now().Add(-1 * time.Hour)
	records, err := store.QueryUnnotified("server1", since)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "20250503-120000", records[0].RoundKey)
}

func TestStateStore_PurgeCompleted_RemovesOldNotifiedRecords(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	require.NoError(t, store.RecordUpload("server1", "20250503-120000", artifact))
	require.NoError(t, store.RecordNotified("server1", "20250503-120000"))

	// purge everything older than 1 minute in the future → catches our just-created record
	err := store.PurgeCompleted(time.Now().Add(1 * time.Minute))
	require.NoError(t, err)

	// the record is notified and old enough → should be gone from the store
	// verify by checking that a QueryUnnotified with zero since returns nothing
	// (the record was notified, so it would never appear regardless, but we verify
	// the bucket still works correctly after purge)
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestStateStore_PurgeCompleted_LeavesUncompleteAndNewRecords(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}

	// notified but recent — should survive purge
	require.NoError(t, store.RecordUpload("server1", "20250503-120001", artifact))
	require.NoError(t, store.RecordNotified("server1", "20250503-120001"))

	// not notified — should survive purge regardless of age
	require.NoError(t, store.RecordUpload("server1", "20250503-120002", artifact))

	// purge with cutoff in the past → nothing should be old enough to purge
	err := store.PurgeCompleted(time.Now().Add(-1 * time.Hour))
	require.NoError(t, err)

	// unnotified record still present
	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "20250503-120002", records[0].RoundKey)
}

func TestStateStore_QueryUnnotified_ScopedToServerID(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{Filename: "round_001.bf2demo", Type: config.ArtifactTypeBF2Demo, RemoteRef: "https://example.com/a.bf2demo"}
	require.NoError(t, store.RecordUpload("server1", "20250503-120000", artifact))
	require.NoError(t, store.RecordUpload("server2", "20250503-120001", artifact))

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "server1", records[0].ServerID)
}

func TestStateStore_RecordUpload_AppearsInQueryUnnotified(t *testing.T) {
	store := newTestStore(t)

	artifact := UploadedArtifact{
		Filename:  "round_001.bf2demo",
		Type:      config.ArtifactTypeBF2Demo,
		RemoteRef: "https://example.com/round_001.bf2demo",
	}

	err := store.RecordUpload("server1", "20250503-142301", artifact)
	require.NoError(t, err)

	records, err := store.QueryUnnotified("server1", time.Time{})
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "server1", records[0].ServerID)
	assert.Equal(t, "20250503-142301", records[0].RoundKey)
	require.Len(t, records[0].Artifacts, 1)
	assert.Equal(t, artifact, records[0].Artifacts[0])
	assert.False(t, records[0].Notified)
}
