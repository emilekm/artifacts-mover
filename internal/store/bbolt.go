package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	bolt "go.etcd.io/bbolt"
)

var (
	ErrRoundExists   = errors.New("store: round already exists")
	ErrRoundNotFound = errors.New("store: round not found")
)

// record is a round plus the enqueue time kept only inside the store.
type record struct {
	Round     types.Round
	CreatedAt time.Time
}

// BboltStore keeps one bucket per server, keyed by round ID.
type BboltStore struct {
	db *bolt.DB
}

func NewBboltStore(path string) (*BboltStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &BboltStore{db: db}, nil
}

func (s *BboltStore) EnqueueRound(r types.Round) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(r.ServerID))
		if err != nil {
			return err
		}

		if b.Get([]byte(r.RoundID)) != nil {
			return fmt.Errorf("%w: %s/%s", ErrRoundExists, r.ServerID, r.RoundID)
		}

		return put(b, record{
			Round:     r,
			CreatedAt: time.Now().UTC(),
		})
	})
}

func (s *BboltStore) PendingUploads() ([]types.Round, error) {
	return s.query(func(rec record) bool {
		return !rec.Round.Uploaded
	})
}

func (s *BboltStore) MarkUploaded(serverID, roundID string, t types.ArtifactType) error {
	return s.update(serverID, roundID, func(rec *record) error {
		artifact, ok := rec.Round.Artifacts[t]
		if !ok {
			return fmt.Errorf("store: round %s/%s has no %s artifact", serverID, roundID, t)
		}
		artifact.Uploaded = true
		rec.Round.Artifacts[t] = artifact
		rec.Round.Uploaded = allUploaded(rec.Round.Artifacts)
		return nil
	})
}

// UnpublishedRounds groups the rounds still waiting to be published by server,
// oldest first. RoundID is a fixed-width UTC RFC3339 timestamp, so sorting on it
// is chronological.
func (s *BboltStore) UnpublishedRounds() (map[string][]types.Round, error) {
	rounds := make(map[string][]types.Round)

	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			serverID := string(name)
			return b.ForEach(func(_, v []byte) error {
				var rec record
				if err := json.Unmarshal(v, &rec); err != nil {
					return err
				}
				if rec.Round.Published {
					return nil
				}
				rounds[serverID] = append(rounds[serverID], rec.Round)
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}

	for _, serverRounds := range rounds {
		slices.SortFunc(serverRounds, func(a, b types.Round) int {
			return strings.Compare(a.RoundID, b.RoundID)
		})
	}
	return rounds, nil
}

func (s *BboltStore) MarkPublished(serverID, roundID string) error {
	return s.update(serverID, roundID, func(rec *record) error {
		rec.Round.Published = true
		rec.Round.FirstFailedAt = time.Time{}
		return nil
	})
}

func (s *BboltStore) RecordFailure(serverID, roundID string) error {
	return s.update(serverID, roundID, func(rec *record) error {
		if rec.Round.FirstFailedAt.IsZero() {
			rec.Round.FirstFailedAt = time.Now().UTC()
		}
		return nil
	})
}

func (s *BboltStore) PurgeCompleted(olderThan time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.ForEach(func(_ []byte, b *bolt.Bucket) error {
			var toDelete [][]byte
			err := b.ForEach(func(k, v []byte) error {
				var rec record
				if err := json.Unmarshal(v, &rec); err != nil {
					return err
				}
				// Publishing no longer waits for the upload, so both have to be
				// done before a record can go; otherwise purging would abandon
				// an upload that is still being retried.
				if rec.Round.Published && rec.Round.Uploaded && rec.CreatedAt.Before(olderThan) {
					toDelete = append(toDelete, k)
				}
				return nil
			})
			if err != nil {
				return err
			}

			for _, k := range toDelete {
				if err := b.Delete(k); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

func (s *BboltStore) Close() error {
	return s.db.Close()
}

// query returns every round matching keep.
func (s *BboltStore) query(keep func(record) bool) ([]types.Round, error) {
	var rounds []types.Round
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(_ []byte, b *bolt.Bucket) error {
			return b.ForEach(func(_, v []byte) error {
				var rec record
				if err := json.Unmarshal(v, &rec); err != nil {
					return err
				}
				if !keep(rec) {
					return nil
				}
				rounds = append(rounds, rec.Round)
				return nil
			})
		})
	})
	if err != nil {
		return nil, err
	}
	return rounds, nil
}

// update applies fn to a stored record and writes it back.
func (s *BboltStore) update(serverID, roundID string, fn func(*record) error) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(serverID))
		if b == nil {
			return fmt.Errorf("%w: %s/%s", ErrRoundNotFound, serverID, roundID)
		}

		raw := b.Get([]byte(roundID))
		if raw == nil {
			return fmt.Errorf("%w: %s/%s", ErrRoundNotFound, serverID, roundID)
		}

		var rec record
		if err := json.Unmarshal(raw, &rec); err != nil {
			return err
		}
		if err := fn(&rec); err != nil {
			return err
		}
		return put(b, rec)
	})
}

func put(b *bolt.Bucket, rec record) error {
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return b.Put([]byte(rec.Round.RoundID), raw)
}

func allUploaded(artifacts map[types.ArtifactType]types.Artifact) bool {
	for _, artifact := range artifacts {
		if !artifact.Uploaded {
			return false
		}
	}
	return true
}
