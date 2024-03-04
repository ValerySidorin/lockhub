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
	AcquireLock(clientID, name string) error
	DeleteLock(clientID, name string) error
}

type InmemStore struct {
	sessions *ttlcache.Cache[string, *dto.Session]
	locks    map[string]*dto.Lock
	smu      sync.RWMutex
	lmu      sync.RWMutex

	l *slog.Logger
}

func NewInmemStore(logger *slog.Logger) *InmemStore {
	s := &InmemStore{
		l: logger,
	}

	sessions := ttlcache.New[string, *dto.Session]()
	sessions.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, *dto.Session]) {
		s.lmu.Lock()
		defer s.lmu.Unlock()

		for _, lock := range i.Value().Locks {
			delete(s.locks, lock)
		}

		s.l.Debug("session expired", "client_id", i.Value().ClientID)
	})
	go sessions.Start()

	return &InmemStore{
		sessions: sessions,
	}
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

func (s *InmemStore) AcquireLock(clientID, name string) error {
	s.lmu.Lock()
	defer s.lmu.Unlock()

	panic("not implemented")
}

func (s *InmemStore) DeleteLock(clientID, name string) error {
	s.smu.Lock()
	s.lmu.Lock()
	defer s.lmu.Unlock()
	defer s.smu.Unlock()

	item := s.sessions.Get(clientID)
	if item != nil {
		session := item.Value()
		lockIdx := -1
		for i, lock := range session.Locks {
			if lock == name {
				lockIdx = i
				break
			}
		}
		if lockIdx != -1 {
			locks := session.Locks[0:lockIdx]
			locks = append(locks, session.Locks[lockIdx+1:]...)
			session.Locks = locks
			s.sessions.Set(clientID, session, item.ExpiresAt().Sub(time.Now()))
		}
		delete(s.locks, name)
	}

	return nil
}
