package cluster

import "context"

var ErrNotLeader = &NotLeaderError{}

type NotLeaderError struct{}
func (e *NotLeaderError) Error() string { return "not leader" }

// Node abstracts a cluster node (leader or follower).
type Node interface {
	// Propose replicates a command (only valid on leader).
	// For single-node, this always succeeds.
	Propose(ctx context.Context, cmd Command) (any, error)

	// Read-only convenience that does NOT mutate state; forwarded to the FSM’s underlying store if needed.
	// (Optional: not required for the first pass; reads can go directly to store package.)
}
