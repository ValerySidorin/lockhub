package service

import (
	"errors"
	"fmt"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/hashicorp/raft"
)

func (s *ServiceImpl) TryAcquireLock(clientID, lockName string, lockVersion uint64) error {
	if s.raft.State() != raft.Leader {
		return ErrNotLeader
	}

	c := command{
		OpCode:      setLockOpCode,
		ClientID:    clientID,
		LockName:    lockName,
		LockVersion: lockVersion,
	}

	s.lmu.Lock()
	defer s.lmu.Unlock()

	lock, ok, err := s.store.GetLock(lockName)
	if err != nil {
		return fmt.Errorf("get lock: %w", err)
	}
	if ok {
		if lock.ClientID == "" {
			if lockVersion < lock.Version {
				return errors.New(
					"requested lock version is lower then already held")
			}

			if err := s.applyCommand(c); err != nil {
				return fmt.Errorf("apply command: %w", err)
			}

			return nil
		}
		if lock.ClientID == clientID {
			return nil
		}
		return errors.New("failed to acquire lock")
	}

	if err := s.applyCommand(c); err != nil {
		return fmt.Errorf("apply command: %w", err)
	}

	return nil
}

func (s *ServiceImpl) ReleaseLock(clientID, lockName string) error {
	s.lmu.Lock()
	defer s.lmu.Unlock()

	l, ok, err := s.store.GetLock(lockName)
	if err != nil {
		return fmt.Errorf("get lock: %w", err)
	}
	if !ok {
		return nil
	}

	if clientID != l.ClientID {
		return fmt.Errorf("attempted to release lock for another session")
	}

	c := command{
		OpCode:   setLockOpCode,
		ClientID: "",
		LockName: lockName,
	}

	if err := s.applyCommand(c); err != nil {
		s.logger.Error(
			fmt.Errorf("apply command: %w", err).Error(),
		)
	}
	return nil
}

func (f *fsm) applySetLock(clientID, name string, version uint64) error {
	l := dto.Lock{
		Name:     name,
		ClientID: clientID,
		Version:  version,
	}
	if err := f.store.SetLock(l); err != nil {
		return fmt.Errorf("service: apply set lock: %w", err)
	}

	return nil
}

func (f *fsm) applyDeleteLock(name string) error {
	if err := f.store.DeleteLock(name); err != nil {
		return fmt.Errorf("service: apply delete lock: %w", err)
	}

	return nil
}
