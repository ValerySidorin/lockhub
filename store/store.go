package store

import (
	"sync"

	"github.com/ValerySidorin/lockhub/internal/dto"
)

type Storer interface {
	GetSession(clientID string) (dto.Session, bool, error)
	SetSession(sess dto.Session) error
	DeleteSession(clientID string) error
	GetLock(name string) (dto.Lock, bool, error)
	SetLock(lock dto.Lock) error
	DeleteLock(name string) error
	Snapshot() Storer
}

type InmemStore struct {
	sessions map[string]dto.Session
	locks    map[string]dto.Lock

	sync.RWMutex
}

func NewInmemStore() *InmemStore {
	return &InmemStore{
		sessions: make(map[string]dto.Session),
		locks:    make(map[string]dto.Lock),
	}
}

func (s *InmemStore) GetSession(clientID string) (dto.Session, bool, error) {
	sess, ok := s.sessions[clientID]
	return sess, ok, nil
}

func (s *InmemStore) SetSession(sess dto.Session) error {
	s.sessions[sess.ClientID] = sess
	return nil
}

func (s *InmemStore) DeleteSession(clientID string) error {
	sess, ok := s.sessions[clientID]
	if !ok {
		return nil
	}

	for _, lock := range sess.Locks {
		copyLock := lock
		copyLock.ClientID = ""
		s.locks[lock.Name] = copyLock
	}
	delete(s.sessions, clientID)

	return nil
}

func (s *InmemStore) GetLock(name string) (dto.Lock, bool, error) {
	lock, ok := s.locks[name]
	return lock, ok, nil
}

func (s *InmemStore) SetLock(lock dto.Lock) error {
	s.Lock()
	defer s.Unlock()

	exLock, ok := s.locks[lock.Name]
	s.locks[lock.Name] = lock

	if lock.ClientID == "" {
		if ok {
			sess, ok := s.sessions[exLock.ClientID]
			if ok {
				delete(sess.Locks, exLock.Name)
			}
		}
	}
	return nil
}

func (s *InmemStore) DeleteLock(name string) error {
	lock, ok := s.locks[name]
	if !ok {
		return nil
	}
	delete(s.locks, name)

	if lock.ClientID == "" {
		return nil
	}

	sess, ok := s.sessions[lock.ClientID]
	if !ok {
		return nil
	}

	delete(sess.Locks, name)
	return nil
}

func (s *InmemStore) Snapshot() Storer {
	s.Lock()
	defer s.Unlock()

	sessionsCopy := make(map[string]dto.Session)
	for k, v := range s.sessions {
		sessionsCopy[k] = v
	}

	locksCopy := make(map[string]dto.Lock)
	for k, v := range s.locks {
		locksCopy[k] = v
	}

	return &InmemStore{
		sessions: sessionsCopy,
		locks:    locksCopy,
	}
}
