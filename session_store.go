package lockhub

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/jellydator/ttlcache/v3"
)

type Storer interface {
	SetSession(s dto.Session, ttl time.Duration) error
	GetLock(name string) (dto.Lock, bool)
	SetLock(lock dto.Lock) error
	DeleteLock(clientID, name string) error
}

type InmemStore struct {
	sessions *ttlcache.Cache[string, dto.Session]
	locks    map[string]dto.Lock
	sync.RWMutex
}

func NewInmemStore() *InmemStore {
	s := &InmemStore{}

	sessions := ttlcache.New[string, dto.Session]()
	sessions.OnEviction(func(ctx context.Context, er ttlcache.EvictionReason, i *ttlcache.Item[string, dto.Session]) {
		s.Lock()
		defer s.Unlock()

		fmt.Println("session evicted")
		for _, lock := range i.Value().Locks {
			delete(s.locks, lock)
		}
	})
	go sessions.Start()

	return &InmemStore{
		sessions: sessions,
	}
}

func (s *InmemStore) SetSession(session dto.Session, ttl time.Duration) error {
	s.Lock()
	defer s.Unlock()
	item := s.sessions.Get(session.ClientID)
	if item != nil {
		existingSess := item.Value()
		session.Locks = existingSess.Locks
	}

	s.sessions.Set(session.ClientID, session, ttl)
	return nil
}

func (s *InmemStore) GetLock(name string) (dto.Lock, bool) {
	s.RLock()
	defer s.RUnlock()

	lock, ok := s.locks[name]
	return lock, ok
}

func (s *InmemStore) SetLock(lock dto.Lock) error {
	s.Lock()
	defer s.Unlock()

	s.locks[lock.Name] = lock
	return nil
}

func (s *InmemStore) DeleteLock(clientID, name string) error {
	s.Lock()
	defer s.Unlock()

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
