package store

import (
	"sync"
	"testing"
	"time"
)

func TestSet(t *testing.T) {
	store := NewStore()

	// set a value 
	err := store.Set("foo", "bar")
	if err != nil {
		t.Fatalf("Unexpected erro: %v", err)
	}

	// Get the value 
	val, ok := store.Get("foo")
	if !ok {
		t.Fatalf("expected key 'foo' to exist")
	}
	if val != "bar" {
		t.Fatalf("expected 'bar', got %v ", val)
	}
}

func TestGetNonExistanceKey(t *testing.T) {
	store := NewStore()

	_, ok := store.Get("doesnotexist")
	if ok {
		t.Fatalf("expected key to not exist")
	}
}

func TestDelete(t *testing.T) {
	s := NewStore()

	s.Set("foo", "bar")

	deleted := s.Delete("foo")
	if !deleted {
		t.Fatalf("expected key to be deleted")
	}

	_, ok := s.Get("foo")
	if ok {
		t.Fatalf("expected key to not exist after deletion")
	}

	// Try deleting a non-existent key 
	deleted = s.Delete("no-there")
	if deleted {
		t.Fatalf("expected false when deleting non-existent key")
	}

}

func TestExists(t *testing.T) {
	s := NewStore()

	s.Set("foo", "bar")

	if !s.Exists("foo") {
		t.Fatalf("expected key to exist")
	}

	if s.Exists("nope") {
		t.Fatalf("expected key to not exist")
	}
}

func TestSetWithTTL(t *testing.T) {
	s:= NewStore()
	s.SetWithTTL("foo", "bar", 100 * time.Millisecond)

	val, ok := s.Get("foo")
	if !ok || val != "bar" {
		t.Fatalf("expected to get value before expire")
	}

	// Waiting for expeiration
	time.Sleep(150 * time.Millisecond)

	_, ok = s.Get("foo")
	if ok {
		t.Fatalf("expected value to be expired and gone")
	}

}

func TestIncrement(t *testing.T) {
	s := NewStore()
	s.Set("counter", "0")

	// Run many increments concurrently 
	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func ()  {
			defer wg.Done()
			s.Incr("counter")
		}()
	}
	wg.Wait()

	val, _ := s.Get("counter")
	if val != "50" {
		t.Fatalf("expected 50, got %s", val)
	}

	s.Decr("counter")
	val, _ = s.Get("counter")
		if val != "49" {
		t.Fatalf("expected 49, got %s", val)
	}
}

func TestCAS(t *testing.T) {
	s := NewStore()
	s.Set("foo", "bar")

	ok := s.CAS("foo", "bar", "baz")
	if !ok {
		t.Fatalf("CAS should succeed when expected matches")
	}

	val, _ := s.Get("foo")
	if val != "baz" {
		t.Fatalf("expected 'baz', got %s", val)
	}

	ok = s.CAS("foo", "notmatch", "xxx")
	if ok {
		t.Fatalf("CAS should fail when expected does not match")
	}
}