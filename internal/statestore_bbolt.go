package internal

import (
	"encoding/json"
	"time"

	bolt "go.etcd.io/bbolt"
)

type bboltStateStore struct {
	db *bolt.DB
}

type roundValue struct {
	ServerID  string             `json:"serverID"`
	RoundKey  string             `json:"roundKey"`
	Artifacts []UploadedArtifact `json:"artifacts"`
	Notified  bool               `json:"notified"`
	CreatedAt time.Time          `json:"createdAt"`
}

func NewBboltStateStore(path string) (StateStore, error) {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return nil, err
	}
	return &bboltStateStore{db: db}, nil
}

func (s *bboltStateStore) RecordUpload(serverID, roundKey string, artifact UploadedArtifact) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(serverID))
		if err != nil {
			return err
		}

		var rv roundValue
		if raw := b.Get([]byte(roundKey)); raw != nil {
			if err := json.Unmarshal(raw, &rv); err != nil {
				return err
			}
		} else {
			rv = roundValue{
				ServerID:  serverID,
				RoundKey:  roundKey,
				CreatedAt: time.Now(),
			}
		}

		rv.Artifacts = append(rv.Artifacts, artifact)

		raw, err := json.Marshal(rv)
		if err != nil {
			return err
		}
		return b.Put([]byte(roundKey), raw)
	})
}

func (s *bboltStateStore) RecordNotified(serverID, roundKey string) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(serverID))
		if b == nil {
			return nil
		}

		raw := b.Get([]byte(roundKey))
		if raw == nil {
			return nil
		}

		var rv roundValue
		if err := json.Unmarshal(raw, &rv); err != nil {
			return err
		}
		rv.Notified = true

		updated, err := json.Marshal(rv)
		if err != nil {
			return err
		}
		return b.Put([]byte(roundKey), updated)
	})
}

func (s *bboltStateStore) QueryUnnotified(serverID string, since time.Time) ([]RoundRecord, error) {
	var records []RoundRecord
	err := s.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(serverID))
		if b == nil {
			return nil
		}
		return b.ForEach(func(_, v []byte) error {
			var rv roundValue
			if err := json.Unmarshal(v, &rv); err != nil {
				return err
			}
			if rv.Notified {
				return nil
			}
			if !since.IsZero() && rv.CreatedAt.Before(since) {
				return nil
			}
			records = append(records, RoundRecord{
				ServerID:  rv.ServerID,
				RoundKey:  rv.RoundKey,
				Artifacts: rv.Artifacts,
				Notified:  rv.Notified,
				CreatedAt: rv.CreatedAt,
			})
			return nil
		})
	})
	return records, err
}

func (s *bboltStateStore) PurgeCompleted(olderThan time.Time) error {
	return s.db.Update(func(tx *bolt.Tx) error {
		return tx.ForEach(func(name []byte, b *bolt.Bucket) error {
			var toDelete [][]byte
			if err := b.ForEach(func(k, v []byte) error {
				var rv roundValue
				if err := json.Unmarshal(v, &rv); err != nil {
					return err
				}
				if rv.Notified && rv.CreatedAt.Before(olderThan) {
					toDelete = append(toDelete, k)
				}
				return nil
			}); err != nil {
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

func (s *bboltStateStore) Close() error {
	return s.db.Close()
}
