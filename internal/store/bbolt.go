package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	bolt "go.etcd.io/bbolt"
)

const (
	maxAttempts = 10
	baseBackoff = 30 * time.Second
	maxBackoff  = 30 * time.Minute
)

var (
	ErrRoundExists   = errors.New("store: round already exists")
	ErrRoundNotFound = errors.New("store: round not found")
)

// record is a round plus the retry bookkeeping kept only inside the store.
type record struct {
	Round         types.Round
	Attempts      int
	NextAttemptAt time.Time
	LastError     string
	Dead          bool
	CreatedAt     time.Time
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
		rec.resetBackoff()
		return nil
	})
}

func (s *BboltStore) PendingNotifications() ([]types.Round, error) {
	return s.query(func(rec record) bool {
		return rec.Round.Uploaded && !rec.Round.Notified
	})
}

func (s *BboltStore) MarkNotified(serverID, roundID string) error {
	return s.update(serverID, roundID, func(rec *record) error {
		rec.Round.Notified = true
		rec.resetBackoff()
		return nil
	})
}

func (s *BboltStore) Backoff(serverID, roundID string, cause error) error {
	return s.update(serverID, roundID, func(rec *record) error {
		rec.Attempts++
		rec.LastError = cause.Error()
		rec.NextAttemptAt = time.Now().UTC().Add(backoffFor(rec.Attempts))
		if rec.Attempts >= maxAttempts {
			rec.Dead = true
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
				if rec.Round.Notified && rec.CreatedAt.Before(olderThan) {
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

// query returns every round that is neither dead nor backing off and matches keep.
func (s *BboltStore) query(keep func(record) bool) ([]types.Round, error) {
	now := time.Now().UTC()

	var rounds []types.Round
	err := s.db.View(func(tx *bolt.Tx) error {
		return tx.ForEach(func(_ []byte, b *bolt.Bucket) error {
			return b.ForEach(func(_, v []byte) error {
				var rec record
				if err := json.Unmarshal(v, &rec); err != nil {
					return err
				}
				if rec.Dead || rec.NextAttemptAt.After(now) || !keep(rec) {
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

func (r *record) resetBackoff() {
	r.Attempts = 0
	r.NextAttemptAt = time.Time{}
	r.LastError = ""
}

func backoffFor(attempts int) time.Duration {
	d := baseBackoff << (attempts - 1)
	if d > maxBackoff || d <= 0 {
		return maxBackoff
	}
	return d
}

func allUploaded(artifacts map[types.ArtifactType]types.Artifact) bool {
	for _, artifact := range artifacts {
		if !artifact.Uploaded {
			return false
		}
	}
	return true
}
