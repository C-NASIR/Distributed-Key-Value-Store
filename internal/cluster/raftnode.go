package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"go.etcd.io/etcd/raft/v3"
	"go.etcd.io/etcd/raft/v3/raftpb"
)

// ----- Payload encoding for raft entries -----

type raftMsgType string

const raftMsgCommand raftMsgType = "cmd"

type raftPayload struct {
	Type raftMsgType `json:"type"`
	ID   uint64      `json:"id"`
	Cmd  Command     `json:"cmd"`
}

// ----- Apply result wiring -----

type applyResult struct {
	val any
	err error
}

// ----- RaftNode: implements Node, drives FSM via etcd/raft -----

type RaftNode struct {
	id   uint64
	fsm  FSM
	node raft.Node

	storage *raft.MemoryStorage

	stop chan struct{}
	done chan struct{}

	// proposal result routing
	reqID   uint64
	pending map[uint64]chan applyResult
	pmu     sync.Mutex

	// leader waiting helpers
	isLeader atomic.Bool
}

// NewRaftSingleNode starts a single-node raft instance and begins ticking/apply loop.
func NewRaftSingleNode(id uint64, fsm FSM) *RaftNode {
	st := raft.NewMemoryStorage()
	cfg := &raft.Config{
		ID:              id,
		ElectionTick:    10,
		HeartbeatTick:   1,
		Storage:         st,
		MaxSizePerMsg:   1 << 20,
		MaxInflightMsgs: 256,
		CheckQuorum:     true,
		PreVote:         true,
	}

	// Single-node cluster: we declare ourselves as the only peer.
	peers := []raft.Peer{{ID: id}}
	nd := raft.StartNode(cfg, peers)

	rn := &RaftNode{
		id:      id,
		fsm:     fsm,
		node:    nd,
		storage: st,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		pending: make(map[uint64]chan applyResult),
	}
	go rn.run()
	return rn
}

func (rn *RaftNode) Close() {
	close(rn.stop)
	<-rn.done
}

// Propose replicates a command and returns the FSM.Apply result once committed.
func (rn *RaftNode) Propose(ctx context.Context, cmd Command) (any, error) {
	// (Optional) If you want leader-only accept:
	// if !rn.IsLeader() { return nil, ErrNotLeader }

	id := atomic.AddUint64(&rn.reqID, 1)
	pl := raftPayload{Type: raftMsgCommand, ID: id, Cmd: cmd}
	data, err := json.Marshal(pl)
	if err != nil {
		return nil, err
	}

	// prepare a waiter before proposing
	ch := make(chan applyResult, 1)
	rn.pmu.Lock()
	rn.pending[id] = ch
	rn.pmu.Unlock()

	// Propose to Raft
	if err := rn.node.Propose(ctx, data); err != nil {
		// fail fast, clean pending
		rn.pmu.Lock()
		delete(rn.pending, id)
		rn.pmu.Unlock()
		return nil, err
	}

	select {
	case res := <-ch:
		return res.val, res.err
	case <-ctx.Done():
		// timed out/canceled: caller stops waiting; result may still apply later
		return nil, ctx.Err()
	}
}

func (rn *RaftNode) IsLeader() bool {
	return rn.isLeader.Load()
}

// ----- Internal event loop -----

func (rn *RaftNode) run() {
	defer close(rn.done)

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rn.node.Tick()

		case rd := <-rn.node.Ready():
			// Persist raft state to storage (we're using MemoryStorage now)
			if err := rn.storage.Append(rd.Entries); err != nil {
				// In production: handle fsync/WAL errors and crash/recover strategy
			}
			if !raft.IsEmptyHardState(rd.HardState) {
				_ = rn.storage.SetHardState(rd.HardState)
			}
			if !raft.IsEmptySnap(rd.Snapshot) {
				_ = rn.storage.ApplySnapshot(rd.Snapshot)
			}

			// Send outbound messages (none in single-node; with transport you'd send here)

			// Apply committed entries to FSM
			for _, ent := range rd.CommittedEntries {
				switch ent.Type {
				case raftpb.EntryConfChange:
					var cc raftpb.ConfChange
					_ = cc.Unmarshal(ent.Data)
					_ = rn.node.ApplyConfChange(cc) // no-op for single-node now

				case raftpb.EntryNormal:
					if len(ent.Data) == 0 {
						break
					}
					rn.applyEntry(ent.Data)
				}
			}

			// Update leadership info
			lead := rd.SoftState != nil && rd.SoftState.Lead == rn.id
			rn.isLeader.Store(lead)

			// Tell raft we're done with this Ready
			rn.node.Advance()

		case <-rn.stop:
			rn.node.Stop()
			return
		}
	}
}

func (rn *RaftNode) applyEntry(data []byte) {
	var pl raftPayload
	if err := json.Unmarshal(data, &pl); err != nil {
		// Unknown payload; drop. You might want metrics/logging.
		return
	}
	switch pl.Type {
	case raftMsgCommand:
		val, err := rn.fsm.Apply(pl.Cmd)
		rn.finish(pl.ID, applyResult{val: val, err: err})
	default:
		// ignore unknown types
	}
}

func (rn *RaftNode) finish(id uint64, res applyResult) {
	rn.pmu.Lock()
	ch, ok := rn.pending[id]
	if ok {
		delete(rn.pending, id)
	}
	rn.pmu.Unlock()
	if ok {
		ch <- res
	}
}

// Optional helper: wait until we observe leadership (handy in tests)
func (rn *RaftNode) AwaitLeader(ctx context.Context) error {
	t := time.NewTicker(20 * time.Millisecond)
	defer t.Stop()
	for {
		if rn.IsLeader() {
			return nil
		}
		select {
		case <-t.C:
		case <-ctx.Done():
			return errors.New("leader not established: " + ctx.Err().Error())
		}
	}
}