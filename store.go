package lockhub

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/jellydator/ttlcache/v3"
)

type Storer interface {
	SetSessionTTL(clientID string, ttl time.Duration) error
	GetOrCreateSession(clientID string, ttl time.Duration) (*dto.Session, error)
	TryAcquireLock(sess *dto.Session, name string, version uint64) error
	ReleaseLock(sess *dto.Session, name string) error
}

type StoreConfig struct {
	DeleteStaleLockInterval  time.Duration
	MinLockRetentionDuration time.Duration
}

type InmemStore struct {
	conf StoreConfig

	sessions *ttlcache.Cache[string, *dto.Session]
	locks    map[string]dto.Lock
	smu      sync.RWMutex
	lmu      sync.RWMutex

	l *slog.Logger
}

func NewInmemStore(ctx context.Context, conf StoreConfig, logger *slog.Logger) *InmemStore {
	s := &InmemStore{
		conf:  conf,
		locks: make(map[string]dto.Lock),
		l:     logger,
	}

	sessions := ttlcache.New[string, *dto.Session]()
	sessions.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *dto.Session]) {
		s.lmu.Lock()
		defer s.lmu.Unlock()

		sess := i.Value()

		for _, lock := range sess.Locks {
			s.locks[lock.Name] = dto.Lock{
				Name:      lock.Name,
				ClientID:  "",
				Version:   lock.Version,
				UpdatedAt: lock.UpdatedAt,
			}
		}

		s.l.Debug("session terminated", "client_id", sess.ClientID)
	})
	go sessions.Start()

	go func() {
		t := time.NewTicker(conf.DeleteStaleLockInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				s.deleteStaleLocks()
			}
		}
	}()

	s.sessions = sessions
	return s
}

func (s *InmemStore) SetSessionTTL(clientID string, ttl time.Duration) error {
	s.smu.Lock()
	defer s.smu.Unlock()
	item := s.sessions.Get(clientID)
	if item == nil {
		return errors.New("session not found")
	}

	existingSess := item.Value()
	s.sessions.Set(clientID, existingSess, ttl)
	return nil
}

func (s *InmemStore) GetOrCreateSession(clientID string, ttl time.Duration) (*dto.Session, error) {
	s.smu.Lock()
	defer s.smu.Unlock()
	item := s.sessions.Get(clientID)
	if item != nil {
		return item.Value(), nil
	}

	session := &dto.Session{ClientID: clientID}
	s.sessions.Set(clientID, session, ttl)
	return session, nil
}

func (s *InmemStore) TryAcquireLock(sess *dto.Session, name string, version uint64) error {
	s.lmu.Lock()
	defer s.lmu.Unlock()

	lock, ok := s.locks[name]
	if ok {
		if lock.ClientID == "" {
			if version < lock.Version {
				return errors.New("requested lock version is lower then already held")
			}

			newLock := dto.Lock{
				Name:      name,
				ClientID:  sess.ClientID,
				Version:   version,
				UpdatedAt: time.Now().Unix(),
			}
			s.locks[name] = newLock

			s.smu.Lock()
			sess.Locks = append(sess.Locks, newLock)
			s.smu.Unlock()
			return nil
		}
		if lock.ClientID == sess.ClientID {
			return nil
		}
		return errors.New("failed to acquire lock")
	}

	newLock := dto.Lock{
		Name:      name,
		ClientID:  sess.ClientID,
		Version:   version,
		UpdatedAt: time.Now().Unix(),
	}
	s.locks[name] = newLock

	s.smu.Lock()
	sess.Locks = append(sess.Locks, newLock)
	s.smu.Unlock()
	return nil
}

func (s *InmemStore) ReleaseLock(sess *dto.Session, name string) error {
	lockIdx := -1
	var lock dto.Lock
	for i, l := range sess.Locks {
		if l.Name == name {
			lock = l
			lockIdx = i
			break
		}
	}
	if lockIdx != -1 {
		locks := sess.Locks[0:lockIdx]
		locks = append(locks, sess.Locks[lockIdx+1:]...)
		sess.Locks = locks
	}

	if lock.ClientID == "" {
		return nil
	}

	lock.ClientID = ""
	lock.UpdatedAt = time.Now().Unix()

	s.lmu.Lock()
	defer s.lmu.Unlock()

	s.locks[name] = lock

	return nil
}

func (s *InmemStore) deleteStaleLocks() {
	s.lmu.Lock()
	defer s.lmu.Unlock()

	timestamp := time.Now()

	for k, v := range s.locks {
		if v.ClientID == "" &&
			timestamp.Sub(time.Unix(v.UpdatedAt, 0)) > s.conf.MinLockRetentionDuration {
			delete(s.locks, k)
		}
	}
}
