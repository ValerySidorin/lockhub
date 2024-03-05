package lockhub

import (
	"errors"
	"fmt"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/ValerySidorin/lockhub/internal/protocol"
)

func (s *Server) handleRequest(session *dto.Session, req protocol.Request) error {
	switch req.Cmd {
	case protocol.KeepaliveCommand:
		return s.handleKeepaliveCommand(session)
	case protocol.TryAcquireLockCommand:
		return s.handleTryAcquireLockCommand(session, req.Payload)
	default:
		return errors.New("unknown command")
	}
}

func (s *Server) handleKeepaliveCommand(session *dto.Session) error {
	if err := s.store.SetSessionTTL(session.ClientID,
		s.conf.KeepaliveInterval+s.conf.SessionRetentionDuration); err != nil {
		return fmt.Errorf("set session: %w", err)
	}
	s.l.Debug("session extended", "client_id", session.ClientID)
	time.Sleep(s.conf.KeepaliveInterval)
	return nil
}

func (s *Server) handleTryAcquireLockCommand(
	session *dto.Session, payload []byte) error {
	req, err := protocol.NewTryAcquireLock(payload)
	if err != nil {
		return fmt.Errorf("parse try acquire lock request: %w", err)
	}

	if err := s.store.TryAcquireLock(session, req.Name, req.Version); err != nil {
		return fmt.Errorf("try acquire lock: %w", err)
	}

	s.l.Debug("lock acquired",
		"client_id", session.ClientID, "name", req.Name, "version", req.Version)
	return nil
}
