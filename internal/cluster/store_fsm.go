package cluster

import (
	"encoding/json"
	"time"

	"github.com/C-NASIR/kvstore/internal/store"
)

type StoreFSM struct {
	S *store.Store
}

func NewStoreFSM(s *store.Store) *StoreFSM { return &StoreFSM{S: s} }

func (f *StoreFSM) Apply(cmd Command) (any, error) {
	switch cmd.Op {
	case OpSet:
		return nil, f.S.Set(cmd.Key, string(cmd.Value))
	case OpSetTTL:
		return nil, f.S.SetWithTTL(cmd.Key, string(cmd.Value), cmd.TTL)
	case OpDel:
		ok := f.S.Del(cmd.Key)
		return ok, nil
	case OpIncr:
		return nil, f.S.Incr(cmd.Key)
	case OpDecr:
		return nil, f.S.Decr(cmd.Key)
	case OpCAS:
		ok := f.S.CAS(cmd.Key, string(cmd.Expected), string(cmd.Value))
		return ok, nil
	default:
		return nil, ErrUnknownOp
	}
}

// Optional sugar for JSON objects:
func MustJSON(v any) []byte {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// Optional helper for TTL convenience:
func TTL(ms int64) time.Duration { return time.Duration(ms) * time.Millisecond }
