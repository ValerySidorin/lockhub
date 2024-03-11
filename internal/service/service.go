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
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/store"
	"github.com/hashicorp/raft"
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
	conf        ServiceConfig
	raft        *raft.Raft
	raftTimeout time.Duration

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

	return s
}

func (s *ServiceImpl) Open(
	nodeID, bindAddr, joinAddr string, raftTimeout time.Duration,
	raftLogStore raft.LogStore, raftStableStore raft.StableStore,
	raftSnapshotStore raft.SnapshotStore,
) error {
	raftCfg := raft.DefaultConfig()
	raftCfg.LocalID = raft.ServerID(nodeID)
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

func (s *ServiceImpl) handleRaftStateChange() {
	for isLeader := range s.raft.LeaderCh() {
		if isLeader {
			f := s.raft.Barrier(s.raftTimeout)
			if f.Error() != nil {
				s.raft.LeadershipTransfer()
				continue
			}
			if err := s.bootstrapTTLCaches(); err != nil {
				s.raft.LeadershipTransfer()
				continue
			}
			continue
		}
		s.dropCaches()
	}
}

func (s *ServiceImpl) bootstrapTTLCaches() error {
	s.logger.Debug("bootstrapping caches")
	defer s.logger.Debug("caches bootstrapped")

	s.smu.Lock()
	s.lmu.Lock()
	defer s.lmu.Unlock()
	defer s.smu.Unlock()

	sessEv := ttlcache.New[string, struct{}]()
	lockEv := ttlcache.New[string, struct{}]()

	if err := loadCache(
		sessEv,
		s.conf.KeepaliveInterval+s.conf.SessionRetentionDuration,
		s.store.GetClientIDs,
	); err != nil {
		return fmt.Errorf("load sessions cache: %w", err)
	}

	if err := loadCache(
		lockEv,
		s.conf.LockRetentionDuration,
		s.store.GetLockNames,
	); err != nil {
		return fmt.Errorf("load locks cache: %w", err)
	}

	sessEv.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, struct{}]) {
		s.logger.Debug("session expired", "client_id", i.Key())
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
					lockEv.Set(lock.Name, struct{}{}, s.conf.LockRetentionDuration)
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

		f := s.raft.Apply(b, s.raftTimeout)
		if f.Error() != nil {
			s.logger.Error(
				fmt.Errorf("on session eviction: raft apply: %w", err).Error())
			return
		}
	})

	lockEv.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, struct{}]) {
		s.logger.Debug("lock retention duration exceeded", "lock_name", i.Key())

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

		f := s.raft.Apply(b, s.raftTimeout)
		if f.Error() != nil {
			s.logger.Error(
				fmt.Errorf("on lock eviction: raft apply: %w", err).Error())
			return
		}
	})

	go sessEv.Start()
	go lockEv.Start()

	s.sessionEvictor = sessEv
	s.lockEvictor = lockEv

	return nil
}

func (s *ServiceImpl) dropCaches() {
	s.logger.Debug("dropping caches", "id")
	defer s.logger.Debug("caches dropped", "id")

	s.sessionEvictor.Stop()
	s.lockEvictor.Stop()

	s.sessionEvictor = nil
	s.lockEvictor = nil
}

func (s *ServiceImpl) applyCommand(c command) error {
	fmt.Println("apply")
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

type fsm ServiceImpl

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

func loadCache(
	cache *ttlcache.Cache[string, struct{}],
	ttl time.Duration,
	getData func() ([]string, error),
) error {
	data, err := getData()
	if err != nil {
		return fmt.Errorf("load cache: %w", err)
	}

	for _, item := range data {
		cache.Set(item, struct{}{}, ttl)
	}

	return nil
}
