package store

import (
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"
)

type entry struct {
	value     []byte
	expiresAt time.Time
}

type Store struct {
	data map[string]entry
	mu   sync.RWMutex
	stop chan struct{}
}

func NewStore() *Store {
	s := &Store{
		data: make(map[string]entry),
		stop: make(chan struct{}),
	}
	go s.cleanupLoop()
	return s
}

func (s *Store) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{value: []byte(value)}
	return nil
}

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{
		value:     []byte(value),
		expiresAt: time.Now().Add(ttl),
	}
	return nil
}

func (s *Store) Get(key string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok {
		return "", false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return "", false
	}
	return string(e.value), true
}

func (s *Store) SetObject(key string, val any) error {
	b, err := json.Marshal(val)
	if err != nil {
		return  err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{value: b}
	return nil
}

func (s *Store) GetObject(key string, target any) (bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.data[key]
	if !ok || (!e.expiresAt.IsZero() && time.Now().After(e.expiresAt)) {
		return false, nil
	}
	if error := json.Unmarshal(e.value, target); error != nil {
		return  false, error
	}
	return true, nil
}

func (s *Store) Del(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.data[key]; exists {
		delete(s.data, key)
		return true
	}
	return false
}

func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, exists := s.data[key]
	if !exists {
		return false
	}
	if !e.expiresAt.IsZero() && time.Now().After(e.expiresAt) {
		return false
	}
	return true
}

func (s *Store) cleanupLoop() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.removeExpired()
		case <-s.stop:
			return
		}
	}
}

func (s *Store) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for k, e := range s.data {
		if !e.expiresAt.IsZero() && now.After(e.expiresAt) {
			delete(s.data, k)
		}
	}
}

func (s *Store) Close() {
	close(s.stop)
}

func (s *Store) Incr(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		s.data[key] = entry{value: []byte("1")}
		return nil
	}

	n, err := strconv.Atoi(string(e.value))
	if err != nil {
		return err
	}
	n++
	e.value = []byte(strconv.Itoa(n))
	s.data[key] = e
	return nil
}


func (s *Store) Decr(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok {
		s.data[key] = entry{value: []byte("-1")}
		return nil
	}

	n, err := strconv.Atoi(string(e.value))
	if err != nil {
		return err
	}
	n--
	e.value = []byte(strconv.Itoa(n))
	s.data[key] = e
	return nil
}

func (s *Store) CAS(key, expected, newVal string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.data[key]
	if !ok || string(e.value) != expected {
		return false
	}

	e.value = []byte(newVal)
	s.data[key] = e
	return true
}

type persistedEntry struct {
	Value []byte `json:"value"`
	ExpiresAt time.Time `json:"expires_at"`
}

func (s *Store) SaveToFile(filename string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]persistedEntry)
	for k, e := range s.data {
		out[k] = persistedEntry {
			Value: e.value,
			ExpiresAt: e.expiresAt,
		}
	}

	data, err := json.MarshalIndent(out, "", " ")
	if err != nil {
		return  err
	}
	return os.WriteFile(filename, data, 0644)
}

func (s *Store) LoadFromFile(filename string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := os.ReadFile(filename)
	if err != nil {
		return  err
	}

	var loaded map[string]persistedEntry
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	for k, v := range loaded {
		s.data[k] = entry{value: v.Value, expiresAt: v.ExpiresAt}
	}
	return  nil
}