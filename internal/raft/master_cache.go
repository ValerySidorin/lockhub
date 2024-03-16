package raft

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/ValerySidorin/lockhub/store"
	"github.com/jellydator/ttlcache/v3"
)

type masterCache struct {
	store store.Storer

	sessionKeepAliveInterval time.Duration
	sessionRetentionDuration time.Duration
	lockRetentionDuration    time.Duration

	isUp bool
	sync.Mutex

	sessions *ttlcache.Cache[string, dto.Session]
	locks    *ttlcache.Cache[string, dto.Lock]

	l *slog.Logger
}

func newMasterCache(store store.Storer,
	ski, srd, lrd time.Duration,
	logger *slog.Logger,
) *masterCache {
	return &masterCache{
		store:                    store,
		sessionKeepAliveInterval: ski,
		sessionRetentionDuration: srd,
		lockRetentionDuration:    lrd,
		l:                        logger,
	}
}

func (c *masterCache) getSession(clientID string) (dto.Session, bool) {
	item := c.sessions.Get(clientID)
	if item != nil {
		return item.Value(), true
	}

	return dto.Session{}, false
}

func (c *masterCache) setSession(s dto.Session) {
	c.sessions.Set(s.ClientID, s,
		c.sessionKeepAliveInterval+c.sessionRetentionDuration)
	if c.lockRetentionDuration > 0 {
		for k, v := range s.Locks {
			c.locks.Set(k, v,
				c.sessionKeepAliveInterval+c.sessionRetentionDuration+c.lockRetentionDuration)
		}
	}
}

func (c *masterCache) load() error {
	c.Lock()
	if c.isUp {
		c.Unlock()
		return nil
	}
	c.isUp = true
	c.Unlock()

	c.l.Debug("master cache loading...")
	defer c.l.Debug("master cache loaded")

	c.sessions = ttlcache.New[string, dto.Session]()
	c.locks = ttlcache.New[string, dto.Lock]()

	if err := c.loadSessions(); err != nil {
		return fmt.Errorf("load sessions: %w", err)
	}

	if err := c.loadLocks(); err != nil {
		return fmt.Errorf("load locks: %w", err)
	}

	c.sessions.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, dto.Session]) {
		c.l.Debug("session expired", "client_id", i.Key())
		if c.lockRetentionDuration > 0 {
			sess, ok, err := c.store.GetSession(i.Key())
			if err != nil {
				c.l.Error(
					fmt.Errorf("on session eviction: get session: %w", err).Error(),
				)
				return
			}

			if ok {
				for _, lock := range sess.Locks {
					c.locks.Set(lock.Name, lock, c.lockRetentionDuration)
				}
			}
		}

		cmd := command{
			OpCode:   deleteSessionOpCode,
			ClientID: i.Key(),
		}

		b, _ := json.Marshal(cmd)

		fmt.Println(b)

		// send to raft applicator

		// f := s.raft.Apply(b, s.raftTimeout)
		// if f.Error() != nil {
		// 	s.logger.Error(
		// 		fmt.Errorf("on session eviction: raft apply: %w", err).Error())
		// 	return
		// }
	})

	c.locks.OnEviction(func(
		ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, dto.Lock]) {
		c.l.Debug("lock retention duration exceeded", "lock_name", i.Key())

		c := command{
			OpCode:   deleteLockOpCode,
			ClientID: i.Key(),
		}

		b, _ := json.Marshal(c)
		fmt.Println(b)

		// send to raft applicator

		// f := s.raft.Apply(b, s.raftTimeout)
		// if f.Error() != nil {
		// 	s.logger.Error(
		// 		fmt.Errorf("on lock eviction: raft apply: %w", err).Error())
		// 	return
		// }
	})

	go c.sessions.Start()
	go c.locks.Start()

	return nil
}

func (c *masterCache) drop() {
	c.Lock()
	if !c.isUp {
		c.Unlock()
		return
	}
	c.isUp = false
	c.Unlock()

	c.l.Debug("master cache dropping...")
	c.l.Debug("master cache loaded")

	c.sessions.Stop()
	c.locks.Stop()

	c.sessions = nil
	c.locks = nil
}

func (c *masterCache) loadSessions() error {
	sessions, err := c.store.GetSessions()
	if err != nil {
		return fmt.Errorf("get sessions: %w", err)
	}

	for _, session := range sessions {
		c.sessions.Set(session.ClientID, session,
			c.sessionKeepAliveInterval+c.sessionRetentionDuration)
	}

	return nil
}

func (c *masterCache) loadLocks() error {
	locks, err := c.store.GetLocks()
	if err != nil {
		return fmt.Errorf("get locks: %w", err)
	}

	for _, lock := range locks {
		c.locks.Set(lock.Name, lock, c.lockRetentionDuration)
	}

	return nil
}
