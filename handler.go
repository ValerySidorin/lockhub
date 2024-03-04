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
	default:
		return errors.New("unknown command")
	}
}

func (s *Server) handleKeepaliveCommand(session *dto.Session) error {
	if err := s.store.SetSessionTTL(session.ClientID,
		s.conf.KeepaliveInterval+s.conf.SessionRetentionPeriod); err != nil {
		return fmt.Errorf("set session: %w", err)
	}
	s.l.Debug("session prolonged", "client_id", session.ClientID)
	time.Sleep(s.conf.KeepaliveInterval)
	return nil
}
