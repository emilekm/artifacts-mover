package store

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/stretchr/testify/mock"
)

type MockStore struct {
	mock.Mock

	mu     sync.Mutex
	rounds map[string]types.Round
}

func NewMockStore() *MockStore {
	return &MockStore{
		rounds: make(map[string]types.Round),
	}
}

func (s *MockStore) EnqueueRound(r types.Round) error {
	args := s.Called(r)
	if err := args.Error(0); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.rounds[mockKey(r.ServerID, r.RoundID)] = r
	return nil
}

func (s *MockStore) PendingUploads() ([]types.Round, error) {
	args := s.Called()
	if err := args.Error(0); err != nil {
		return nil, err
	}
	return s.query(func(r types.Round) bool {
		return !r.Uploaded
	}), nil
}

func (s *MockStore) MarkUploaded(serverID, roundID string, t types.ArtifactType) error {
	args := s.Called(serverID, roundID, t)
	if err := args.Error(0); err != nil {
		return err
	}
	return s.update(serverID, roundID, func(r *types.Round) error {
		artifact, ok := r.Artifacts[t]
		if !ok {
			return fmt.Errorf("store: round %s/%s has no %s artifact", serverID, roundID, t)
		}
		artifact.Uploaded = true
		r.Artifacts[t] = artifact
		r.Uploaded = allUploaded(r.Artifacts)
		return nil
	})
}

// UnpublishedRounds groups by server and orders by RoundID, the same contract
// the bbolt store provides.
func (s *MockStore) UnpublishedRounds() (map[string][]types.Round, error) {
	args := s.Called()
	if err := args.Error(0); err != nil {
		return nil, err
	}

	rounds := make(map[string][]types.Round)
	for _, round := range s.query(func(r types.Round) bool { return !r.Published }) {
		rounds[round.ServerID] = append(rounds[round.ServerID], round)
	}
	for _, serverRounds := range rounds {
		slices.SortFunc(serverRounds, func(a, b types.Round) int {
			return strings.Compare(a.RoundID, b.RoundID)
		})
	}
	return rounds, nil
}

func (s *MockStore) MarkPublished(serverID, roundID string) error {
	args := s.Called(serverID, roundID)
	if err := args.Error(0); err != nil {
		return err
	}
	return s.update(serverID, roundID, func(r *types.Round) error {
		r.Published = true
		r.FirstFailedAt = time.Time{}
		return nil
	})
}

func (s *MockStore) RecordFailure(serverID, roundID string) error {
	args := s.Called(serverID, roundID)
	if err := args.Error(0); err != nil {
		return err
	}
	return s.update(serverID, roundID, func(r *types.Round) error {
		if r.FirstFailedAt.IsZero() {
			r.FirstFailedAt = time.Now().UTC()
		}
		return nil
	})
}

// PurgeCompleted drops every published and uploaded round; olderThan is only
// asserted on, since the mock does not track creation times.
func (s *MockStore) PurgeCompleted(olderThan time.Time) error {
	args := s.Called(olderThan)
	if err := args.Error(0); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for key, round := range s.rounds {
		if round.Published && round.Uploaded {
			delete(s.rounds, key)
		}
	}
	return nil
}

// Rounds returns a snapshot of the recorded rounds, keyed by "serverID/roundID".
func (s *MockStore) Rounds() map[string]types.Round {
	s.mu.Lock()
	defer s.mu.Unlock()

	rounds := make(map[string]types.Round, len(s.rounds))
	for key, round := range s.rounds {
		rounds[key] = round
	}
	return rounds
}

// query returns matching rounds ordered by key, so assertions stay stable.
func (s *MockStore) query(keep func(types.Round) bool) []types.Round {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]string, 0, len(s.rounds))
	for key, round := range s.rounds {
		if keep(round) {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)

	rounds := make([]types.Round, 0, len(keys))
	for _, key := range keys {
		rounds = append(rounds, s.rounds[key])
	}
	return rounds
}

func (s *MockStore) update(serverID, roundID string, fn func(*types.Round) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := mockKey(serverID, roundID)
	round, ok := s.rounds[key]
	if !ok {
		return fmt.Errorf("%w: %s/%s", ErrRoundNotFound, serverID, roundID)
	}
	if err := fn(&round); err != nil {
		return err
	}
	s.rounds[key] = round
	return nil
}

func mockKey(serverID, roundID string) string {
	return serverID + "/" + roundID
}
