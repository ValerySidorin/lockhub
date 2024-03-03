package lockhub

import (
	"errors"
	"fmt"
	"time"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/ValerySidorin/lockhub/internal/protocol"
)

func (s *Server) handleRequest(req protocol.Request) error {
	switch req.Cmd {
	case protocol.KeepaliveCommand:
		return s.handleKeepaliveCommand(req.Payload)
	default:
		return errors.New("unknown command")
	}
}

func (s *Server) handleKeepaliveCommand(p []byte) error {
	keepalive, err := protocol.NewKeepalive(p)
	if err != nil {
		return fmt.Errorf("new keepalive: %w", err)
	}
	sess := dto.Session{ClientID: keepalive.ClientID}
	if err := s.store.SetSession(sess,
		s.conf.KeepaliveInterval+s.conf.SessionRetentionPeriod); err != nil {
		return fmt.Errorf("set session: %w", err)
	}
	s.l.Info("updated session", "client_id", sess.ClientID)
	time.Sleep(s.conf.KeepaliveInterval)
	return nil
}
