package store

import (
	"strconv"
	"sync"
	"time"
)

type entry struct {
	value     string
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
	s.data[key] = entry{value: value}
	return nil
}

func (s *Store) SetWithTTL(key, value string, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = entry{
		value:     value,
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
		// expired
		return "", false
	}
	return e.value, true
}

func (s *Store) Delete(key string) bool {
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
		s.data[key] = entry{value: "1"}
		return nil
	}

	n, err := strconv.Atoi(e.value)
	if err != nil {
		return  err
	}

	n++
	e.value = strconv.Itoa(n)
	s.data[key] = e
	return  nil
}

func (s *Store) Decr(key string) error {
		s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok {
		s.data[key] = entry{value: "-1"}
		return nil
	}
	n, err := strconv.Atoi(e.value)
	if err != nil {
		return err
	}
	n--
	e.value = strconv.Itoa(n)
	s.data[key] = e
	return nil
}

func (s *Store) CAS(key, expected, newVal string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	e, ok := s.data[key]
	if !ok || e.value != expected {
		return  false
	}
	e.value = newVal
	s.data[key] = e 
	return  true
}