package cluster

// FSM (Finite-State Machine) is where the replicated log is applied.
// It adapts our Store behind a stable interface.
type FSM interface {
	Apply(cmd Command) (any, error)
}
