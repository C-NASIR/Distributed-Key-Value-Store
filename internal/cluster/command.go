package cluster

import "time"

// Op is a replicated operation that mutates the KV state.
type Op string

const (
	OpSet     Op = "SET"
	OpSetTTL  Op = "SET_TTL"
	OpDel     Op = "DEL"
	OpIncr    Op = "INCR"
	OpDecr    Op = "DECR"
	OpCAS     Op = "CAS"
	// (room for more ops)
)

type Command struct {
	Op       Op
	Key      string
	Value    []byte        // used by SET/SET_TTL/CAS new value
	Expected []byte        // used by CAS expected
	TTL      time.Duration // used by SET_TTL
}
