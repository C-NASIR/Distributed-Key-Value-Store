package cluster

import (
	"context"
	"errors"
	"sync"
)

// SingleNode is a trivial "cluster" of size 1.
// It appends to an in-memory log and applies to the FSM immediately.
type SingleNode struct {
	mu   sync.Mutex
	log  []Command
	fsm  FSM
	// later: snapshots, log indexes, WAL, etc.
}

func NewSingleNode(fsm FSM) *SingleNode {
	return &SingleNode{
		fsm: fsm,
		log: make([]Command, 0, 1024),
	}
}

func (n *SingleNode) Propose(ctx context.Context, cmd Command) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	n.mu.Lock()
	n.log = append(n.log, cmd)
	n.mu.Unlock()

	// Apply immediately (single-node "replication").
	return n.fsm.Apply(cmd)
}

// Exported for tests/future: return copy of log length for assertions.
func (n *SingleNode) LogLen() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.log)
}

var ErrUnknownOp = errors.New("unknown op")
