package cluster

import (
	"context"
	"testing"
	"time"

	"github.com/C-NASIR/kvstore/internal/store"
)

func TestRaftSingleNode_SetAndGet(t *testing.T) {
	st := store.NewStore()
	fsm := NewStoreFSM(st)
	rn := NewRaftSingleNode(1, fsm)
	defer rn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := rn.AwaitLeader(ctx); err != nil {
		t.Fatalf("leader wait: %v", err)
	}

	_, err := rn.Propose(ctx, Command{Op: OpSet, Key: "x", Value: []byte("42")})
	if err != nil {
		t.Fatalf("propose set: %v", err)
	}

	v, ok := st.Get("x")
	if !ok || v != "42" {
		t.Fatalf("expected 42, got %q ok=%v", v, ok)
	}
}

func TestRaftSingleNode_CAS_TTL_Incr(t *testing.T) {
	st := store.NewStore()
	fsm := NewStoreFSM(st)
	rn := NewRaftSingleNode(1, fsm)
	defer rn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rn.AwaitLeader(ctx); err != nil {
		t.Fatalf("leader wait: %v", err)
	}

	// SET user=alice
	if _, err := rn.Propose(ctx, Command{Op: OpSet, Key: "user", Value: []byte("alice")}); err != nil {
		t.Fatal(err)
	}

	// CAS(user, alice -> bob)
	out, err := rn.Propose(ctx, Command{Op: OpCAS, Key: "user", Expected: []byte("alice"), Value: []byte("bob")})
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := out.(bool); !ok {
		t.Fatalf("expected CAS success")
	}

	// INCR counter 50 times
	for i := 0; i < 50; i++ {
		if _, err := rn.Propose(ctx, Command{Op: OpIncr, Key: "counter"}); err != nil {
			t.Fatal(err)
		}
	}
	if v, ok := st.Get("counter"); !ok || v != "50" {
		t.Fatalf("expected counter=50, got %q ok=%v", v, ok)
	}

	// SET_TTL tmp=ok, ttl=80ms
	if _, err := rn.Propose(ctx, Command{Op: OpSetTTL, Key: "tmp", Value: []byte("ok"), TTL: 30 * time.Millisecond}); err != nil {
		t.Fatal(err)
	}
	if v, ok := st.Get("tmp"); !ok || v != "ok" {
		t.Fatalf("expected ok before expiry, got %q ok=%v", v, ok)
	}
	time.Sleep(50 * time.Millisecond)
	if _, ok := st.Get("tmp"); ok {
		t.Fatalf("expected tmp expired")
	}
}
