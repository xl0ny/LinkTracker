package application

import (
	"net/url"
	"strings"
	"sync"
)

type TrackPhase int

const (
	TrackPhaseLink TrackPhase = iota
	TrackPhaseTags
)

type TrackState struct {
	Phase TrackPhase
	Link  string
}

type TrackStateStore interface {
	Get(chatID int64) *TrackState
	Set(chatID int64, st *TrackState)
	Clear(chatID int64)
}

type memoryTrackStateStore struct {
	mu    sync.RWMutex
	state map[int64]*TrackState
}

func NewTrackStateStore() TrackStateStore {
	return &memoryTrackStateStore{state: make(map[int64]*TrackState)}
}

func (s *memoryTrackStateStore) Get(chatID int64) *TrackState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state[chatID]
}

func (s *memoryTrackStateStore) Set(chatID int64, st *TrackState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st == nil {
		delete(s.state, chatID)
	} else {
		s.state[chatID] = st
	}
}

func (s *memoryTrackStateStore) Clear(chatID int64) {
	s.Set(chatID, nil)
}

func IsValidLink(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return (u.Scheme == "http" || u.Scheme == "https") &&
		(u.Host == "github.com" || u.Host == "stackoverflow.com" || strings.HasSuffix(u.Host, ".stackoverflow.com"))
}

func ParseTags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}
