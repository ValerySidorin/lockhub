package lockhub

import (
	"errors"
	"fmt"

	"github.com/ValerySidorin/lockhub/internal/protocol"
)

func (s *Server) handleRequest(clientID string, req protocol.Request) error {
	switch req.Cmd {
	case protocol.KeepaliveOpCode:
		return s.handleKeepaliveCommand(clientID)
	case protocol.TryAcquireLockOpCode:
		return s.handleTryAcquireLockCommand(clientID, req.Payload)
	case protocol.ReleaseLockOpCode:
		return s.handleReleaseLockCommand(clientID, req.Payload)
	default:
		return errors.New("unknown command")
	}
}

func (s *Server) handleKeepaliveCommand(clientID string) error {
	if err := s.service.ExtendSession(clientID); err != nil {
		return fmt.Errorf("extend session: %w", err)
	}
	s.l.Debug("session extended", "client_id", clientID)
	return nil
}

func (s *Server) handleTryAcquireLockCommand(
	clientID string, payload []byte) error {
	req, err := protocol.NewTryAcquireLock(payload)
	if err != nil {
		return fmt.Errorf("parse try acquire lock request: %w", err)
	}

	if err := s.service.TryAcquireLock(clientID, req.Name, req.Version); err != nil {
		return fmt.Errorf("try acquire lock: %w", err)
	}

	s.l.Debug("lock acquired",
		"client_id", clientID, "name", req.Name, "version", req.Version)
	return nil
}

func (s *Server) handleReleaseLockCommand(clientID string, payload []byte) error {
	req, err := protocol.NewReleaseLock(payload)
	if err != nil {
		return fmt.Errorf("parse release request: %w", err)
	}

	if err := s.service.ReleaseLock(clientID, req.Name); err != nil {
		return fmt.Errorf("release lock: %w", err)
	}

	s.l.Debug("lock released", "client_id", clientID, "name", req.Name)
	return nil
}
