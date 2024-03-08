package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/store"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/jellydator/ttlcache/v3"
)

var (
	ErrNotLeader = errors.New("not leader")
)

type Service interface {
	ExtendSession(clientID string) error
	CreateSessionIfNotExists(clientID string) error
	TryAcquireLock(clientID, lockName string, lockVersion uint64) error
	ReleaseLock(clientID, lockName string) error
}

type ServiceImpl struct {
	conf ServiceConfig
	raft *raft.Raft

	store store.Storer

	smu sync.RWMutex
	lmu sync.Mutex

	sessionEvictor *ttlcache.Cache[string, struct{}]
	lockEvictor    *ttlcache.Cache[string, struct{}]

	logger *slog.Logger
}

func New(conf ServiceConfig, store store.Storer, l *slog.Logger) *ServiceImpl {
	s := &ServiceImpl{
		conf:   conf,
		store:  store,
		logger: l,
	}

	se := ttlcache.New[string, struct{}]()
	le := ttlcache.New[string, struct{}]()

	se.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, struct{}]) {
		if s.conf.LockRetentionDuration > 0 {
			sess, ok, err := s.store.GetSession(i.Key())
			if err != nil {
				s.logger.Error(
					fmt.Errorf("on session eviction: get session: %w", err).Error(),
				)
				return
			}

			if ok {
				for _, lock := range sess.Locks {
					le.Set(lock.Name, struct{}{}, s.conf.LockRetentionDuration)
				}
			}
		}

		c := command{
			OpCode:   deleteSessionOpCode,
			ClientID: i.Key(),
		}

		b, err := json.Marshal(c)
		if err != nil {
			s.logger.Error(
				fmt.Errorf("on session eviction: marshal: %w", err).Error())
			return
		}

		f := s.raft.Apply(b, raftTimeout)
		if f.Error() != nil {
			s.logger.Error(
				fmt.Errorf("on session eviction: raft apply: %w", err).Error())
			return
		}
	})

	le.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, struct{}]) {
		c := command{
			OpCode:   deleteLockOpCode,
			ClientID: i.Key(),
		}

		b, err := json.Marshal(c)
		if err != nil {
			s.logger.Error(
				fmt.Errorf("on lock eviction: marshal: %w", err).Error())
			return
		}

		f := s.raft.Apply(b, raftTimeout)
		if f.Error() != nil {
			s.logger.Error(
				fmt.Errorf("on lock eviction: raft apply: %w", err).Error())
			return
		}
	})

	go se.Start()
	go le.Start()

	s.sessionEvictor = se
	s.lockEvictor = le

	return s
}

func (s *ServiceImpl) Open() error {
	raftCfg := raft.DefaultConfig()
	raftCfg.LocalID = raft.ServerID(s.conf.Raft.NodeID)

	addr, err := net.ResolveTCPAddr("tcp", s.conf.Raft.Bind)
	if err != nil {
		return fmt.Errorf("resolve raft bind addr: %w", err)
	}

	transport, err := raft.NewTCPTransport(s.conf.Raft.Bind, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("init raft transport: %w", err)
	}

	snapshots, err := raft.NewFileSnapshotStore(s.conf.Raft.Dir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("init file snapshot store: %w", err)
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore
	db, err := raftboltdb.New(raftboltdb.Options{
		Path: filepath.Join(s.conf.Raft.Dir, "raft.db"),
	})
	if err != nil {
		return fmt.Errorf("init raft store: %w", err)
	}

	logStore = db
	stableStore = db

	r, err := raft.NewRaft(raftCfg, (*fsm)(s), logStore, stableStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("init raft: %w", err)
	}
	s.raft = r

	if s.conf.Raft.Join == "" {
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

	return nil
}

func (s *ServiceImpl) Join(nodeID, addr string) error {
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

func (s *ServiceImpl) applyCommand(c command) error {
	b, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal {%d}: %w", c.OpCode, err)
	}

	f := s.raft.Apply(b, raftTimeout)
	if f.Error() != nil {
		return fmt.Errorf("apply {%d}: %w", c.OpCode, err)
	}

	return nil
}

type fsm ServiceImpl

func (f *fsm) Apply(l *raft.Log) interface{} {
	var c command
	if err := json.Unmarshal(l.Data, &c); err != nil {
		panic(fmt.Sprintf("unmarshal command: %s", err.Error()))
	}

	switch c.OpCode {
	case setSessionOpCode:
		return f.applySetSession(c.Session)
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
