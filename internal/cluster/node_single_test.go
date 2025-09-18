package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/C-NASIR/kvstore/internal/store"
)

func TestSingleNodeSetAndGetThroughFSM(t *testing.T) {
	st := store.NewStore()
	fsm := NewStoreFSM(st)
	node := NewSingleNode(fsm)

	_, err := node.Propose(context.Background(), Command{
		Op:    OpSet,
		Key:   "x",
		Value: []byte("42"),
	})
	if err != nil {
		t.Fatalf("propose SET failed: %v", err)
	}

	val, ok := st.Get("x")
	if !ok || val != "42" {
		t.Fatalf("expected 42, got %q ok=%v", val, ok)
	}
	if node.LogLen() != 1 {
		t.Fatalf("expected log len 1, got %d", node.LogLen())
	}
}

func TestSingleNodeCASAndTTL(t *testing.T) {
	st := store.NewStore()
	fsm := NewStoreFSM(st)
	node := NewSingleNode(fsm)

	_, err := node.Propose(context.Background(), Command{
		Op:    OpSet,
		Key:   "user",
		Value: []byte("alice"),
	})
	if err != nil { t.Fatal(err) }

	// CAS success
	out, err := node.Propose(context.Background(), Command{
		Op:       OpCAS,
		Key:      "user",
		Expected: []byte("alice"),
		Value:    []byte("bob"),
	})
	if err != nil { t.Fatal(err) }
	if ok, _ := out.(bool); !ok {
		t.Fatalf("CAS should succeed")
	}

	// Set with TTL, ensure expiry
	_, err = node.Propose(context.Background(), Command{
		Op:    OpSetTTL,
		Key:   "tmp",
		Value: []byte("keep-me"),
		TTL:   80 * time.Millisecond,
	})
	if err != nil { t.Fatal(err) }

	if v, ok := st.Get("tmp"); !ok || v != "keep-me" {
		t.Fatalf("expected keep-me before expiry, got %q ok=%v", v, ok)
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := st.Get("tmp"); ok {
		t.Fatalf("expected tmp to be expired")
	}
}
