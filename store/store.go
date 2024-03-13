package store

import (
	"errors"
	"sync"

	"github.com/ValerySidorin/lockhub/internal/dto"
)

type Storer interface {
	// For cache loader
	GetSessions() ([]dto.Session, error)
	GetLocks() ([]dto.Lock, error)

	GetSession(clientID string) (dto.Session, bool, error)
	SetSessionIfNotExists(clientID string) error
	DeleteSession(clientID string) error

	GetLock(name string) (dto.Lock, bool, error)
	SetLock(lock dto.Lock) error
	DeleteLock(name string) error

	// For raft
	Snapshot() Storer
}

type InmemStore struct {
	Sessions map[string]dto.Session `json:"sessions"`
	Locks    map[string]dto.Lock    `json:"locks"`

	sync.RWMutex `json:"-"`
}

func NewInmemStore() *InmemStore {
	return &InmemStore{
		Sessions: make(map[string]dto.Session),
		Locks:    make(map[string]dto.Lock),
	}
}

func (s *InmemStore) GetSessions() ([]dto.Session, error) {
	res := make([]dto.Session, 0, len(s.Sessions))
	for _, v := range s.Sessions {
		res = append(res, v)
	}

	return res, nil
}

func (s *InmemStore) GetLocks() ([]dto.Lock, error) {
	res := make([]dto.Lock, 0, len(s.Locks))
	for _, v := range s.Locks {
		res = append(res, v)
	}

	return res, nil
}

func (s *InmemStore) GetSession(clientID string) (dto.Session, bool, error) {
	sess, ok := s.Sessions[clientID]
	return sess, ok, nil
}

func (s *InmemStore) SetSessionIfNotExists(clientID string) error {
	_, ok := s.Sessions[clientID]
	if ok {
		return nil
	}
	s.Sessions[clientID] = dto.Session{
		ClientID: clientID,
		Locks:    make(map[string]dto.Lock),
	}
	return nil
}

func (s *InmemStore) DeleteSession(clientID string) error {
	sess, ok := s.Sessions[clientID]
	if !ok {
		return nil
	}

	for _, lock := range sess.Locks {
		copyLock := lock
		copyLock.ClientID = ""
		s.Locks[lock.Name] = copyLock
	}
	delete(s.Sessions, clientID)

	return nil
}

func (s *InmemStore) GetLock(name string) (dto.Lock, bool, error) {
	lock, ok := s.Locks[name]
	return lock, ok, nil
}

func (s *InmemStore) SetLock(lock dto.Lock) error {
	s.Lock()
	defer s.Unlock()

	exLock, ok := s.Locks[lock.Name]
	s.Locks[lock.Name] = lock

	if lock.ClientID == "" {
		if ok {
			sess, ok := s.Sessions[exLock.ClientID]
			if ok {
				delete(sess.Locks, exLock.Name)
			}
		}

		return nil
	}

	sess, ok := s.Sessions[lock.ClientID]
	if !ok {
		return errors.New("session not found")
	}

	sess.Locks[lock.Name] = lock
	s.Sessions[lock.ClientID] = sess

	return nil
}

func (s *InmemStore) DeleteLock(name string) error {
	lock, ok := s.Locks[name]
	if !ok {
		return nil
	}
	delete(s.Locks, name)

	if lock.ClientID == "" {
		return nil
	}

	sess, ok := s.Sessions[lock.ClientID]
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
	for k, v := range s.Sessions {
		sessionsCopy[k] = v
	}

	locksCopy := make(map[string]dto.Lock)
	for k, v := range s.Locks {
		locksCopy[k] = v
	}

	return &InmemStore{
		Sessions: sessionsCopy,
		Locks:    locksCopy,
	}
}
