package service

import (
	"fmt"
	"time"

	"github.com/hashicorp/raft"
)

func (s *ServiceImpl) ExtendSession(clientID string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	s.sessionEvictor.Set(clientID, struct{}{},
		s.conf.KeepaliveInterval+s.conf.SessionRetentionDuration)

	time.Sleep(s.conf.KeepaliveInterval)

	return nil
}

func (s *ServiceImpl) CreateSessionIfNotExists(clientID string) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	s.smu.Lock()
	defer s.smu.Unlock()

	existingSess, ok, err := s.store.GetSession(clientID)
	if err != nil {
		return fmt.Errorf("get session: %w", err)
	}

	if ok {
		s.sessionEvictor.Set(clientID, struct{}{},
			s.conf.KeepaliveInterval+s.conf.SessionRetentionDuration)
		if s.conf.LockRetentionDuration > 0 {
			for k := range existingSess.Locks {
				s.lockEvictor.Set(k, struct{}{},
					s.conf.KeepaliveInterval+s.conf.LockRetentionDuration)
			}
		}
		return nil
	}

	c := command{
		OpCode:   setSessionIfNotExistsOpCode,
		ClientID: clientID,
	}

	if err := s.applyCommand(c); err != nil {
		return fmt.Errorf("apply command: %w", err)
	}

	s.sessionEvictor.Set(clientID, struct{}{},
		s.conf.KeepaliveInterval+s.conf.SessionRetentionDuration)

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
