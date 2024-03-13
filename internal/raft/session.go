package raft

import (
	"fmt"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/hashicorp/raft"
)

func (s *RaftServiceImpl) ExtendSession(clientID string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	s.masterCache.setSession(dto.Session{ClientID: clientID})
	time.Sleep(s.conf.SessionKeepAliveInterval)

	return nil
}

func (s *RaftServiceImpl) CreateSessionIfNotExists(clientID string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	s.smu.Lock()
	defer s.smu.Unlock()

	_, ok := s.masterCache.getSession(clientID)
	if ok {
		return nil
	}

	c := command{
		OpCode:   setSessionIfNotExistsOpCode,
		ClientID: clientID,
	}

	if err := s.applyCommand(c); err != nil {
		return fmt.Errorf("apply command: %w", err)
	}

	s.masterCache.setSession(dto.Session{ClientID: clientID})

	return nil
}

func (f *fsm) applySetSessionIfNotExists(clientID string) error {
	if err := f.store.SetSessionIfNotExists(clientID); err != nil {
		return fmt.Errorf("fsm: apply set session if not exists: %w", err)
	}

	return nil
}

func (f *fsm) applyDeleteSession(clientID string) error {
	if err := f.store.DeleteSession(clientID); err != nil {
		return fmt.Errorf("fsm: apply set session: %w", err)
	}

	return nil
}
