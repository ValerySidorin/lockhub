package raft

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/store"
	"github.com/ValerySidorin/shclog"
	"github.com/hashicorp/raft"
)

var (
	ErrNotLeader = errors.New("not leader")
)

type RaftService interface {
	ExtendSession(clientID string) error
	CreateSessionIfNotExists(clientID string) error
	TryAcquireLock(clientID, lockName string, lockVersion uint64) error
	ReleaseLock(clientID, lockName string) error
}

type RaftServiceImpl struct {
	conf RaftServiceConfig
	raft *raft.Raft

	raftTimeout time.Duration

	store store.Storer

	smu sync.RWMutex
	lmu sync.Mutex

	masterCache *masterCache

	logger *slog.Logger
}

func NewRaftService(
	conf RaftServiceConfig, store store.Storer, l *slog.Logger,
) *RaftServiceImpl {
	s := &RaftServiceImpl{
		conf:   conf,
		store:  store,
		logger: l,
		masterCache: newMasterCache(
			store, conf.SessionKeepAliveInterval,
			conf.SessionRetentionDuration, conf.LockRetentionDuration, l),
	}

	return s
}

func (s *RaftServiceImpl) Open(
	nodeID, bindAddr, joinAddr string, raftTimeout time.Duration,
	raftLogStore raft.LogStore, raftStableStore raft.StableStore,
	raftSnapshotStore raft.SnapshotStore, logger *slog.Logger,
) error {
	raftCfg := raft.DefaultConfig()
	raftCfg.LocalID = raft.ServerID(nodeID)
	raftCfg.Logger = shclog.New(logger)
	s.raftTimeout = raftTimeout

	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("resolve raft bind addr: %w", err)
	}

	transport, err := raft.NewTCPTransport(bindAddr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("init raft transport: %w", err)
	}

	r, err := raft.NewRaft(raftCfg, (*fsm)(s),
		raftLogStore, raftStableStore, raftSnapshotStore,
		transport)
	if err != nil {
		return fmt.Errorf("init raft: %w", err)
	}
	s.raft = r

	if joinAddr == "" {
		config := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      raftCfg.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		r.BootstrapCluster(config)
	}

	go s.handleRaftStateChange()

	return nil
}

func (s *RaftServiceImpl) Join(nodeID, addr string) error {
	logger := s.logger.With("node_id", nodeID, "addr", addr)
	logger.Debug("received join request")

	configFuture := s.raft.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		err = fmt.Errorf("get raft configuration: %w", err)
		s.logger.Error(err.Error())
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) || srv.Address == raft.ServerAddress(addr) {
			if srv.Address == raft.ServerAddress(addr) && srv.ID == raft.ServerID(nodeID) {
				s.logger.Info("node is already a member of cluster, ignoring join request")
				return nil
			}

			future := s.raft.RemoveServer(srv.ID, 0, 0)
			if err := future.Error(); err != nil {
				return fmt.Errorf("remove server: %w", err)
			}
		}
	}

	f := s.raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(addr), 0, 0)
	if f.Error() != nil {
		return fmt.Errorf("add voter: %w", f.Error())
	}

	logger.Info("node joined cluster")
	return nil
}

func (s *RaftServiceImpl) handleRaftStateChange() {
	for isLeader := range s.raft.LeaderCh() {
		if isLeader {
			f := s.raft.Barrier(s.raftTimeout)
			if f.Error() != nil {
				s.raft.LeadershipTransfer()
				continue
			}
			if err := s.masterCache.load(); err != nil {
				s.raft.LeadershipTransfer()
				continue
			}
			continue
		}
		s.masterCache.drop()
	}
}

func (s *RaftServiceImpl) applyCommand(c command) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal {%d}: %w", c.OpCode, err)
	}

	f := s.raft.Apply(b, s.raftTimeout)
	if f.Error() != nil {
		err = fmt.Errorf("apply {%d}: %w", c.OpCode, err)
		s.logger.Error(err.Error())
		s.raft.LeadershipTransfer()
		return err
	}

	return nil
}

type fsm RaftServiceImpl

func (f *fsm) Apply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("unmarshal command: %s", err.Error()))
	}

	switch c.OpCode {
	case setSessionIfNotExistsOpCode:
		return f.applySetSessionIfNotExists(c.ClientID)
	case deleteSessionOpCode:
		return f.applyDeleteSession(c.ClientID)
	case setLockOpCode:
		return f.applySetLock(c.ClientID, c.LockName, c.LockVersion)
	case deleteLockOpCode:
		return f.applyDeleteLock(c.LockName)
	default:
		panic(fmt.Sprintf("invalid opcode: %d", c.OpCode))
	}
}

func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{
		store:  f.store.Snapshot(),
		logger: f.logger,
	}, nil
}

func (f *fsm) Restore(rc io.ReadCloser) error {
	var s store.Storer
	if err := json.NewDecoder(rc).Decode(s); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}

	f.store = s
	return nil
}

// FSM Snapshot

type fsmSnapshot struct {
	store  store.Storer
	logger *slog.Logger
}

func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	err := func() error {
		b, err := json.Marshal(f.store)
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		if _, err := sink.Write(b); err != nil {
			return fmt.Errorf("write: %w", err)
		}

		return sink.Close()
	}()

	if err != nil {
		if err := sink.Cancel(); err != nil {
			err = fmt.Errorf("cancel: %w", err)
			f.logger.Error(err.Error())
		}
	}

	return err
}

func (f *fsmSnapshot) Release() {}
