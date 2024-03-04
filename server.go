package lockhub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ValerySidorin/lockhub/internal/dto"
	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

type Server struct {
	conf  *ServerConfig
	store Storer
	l     *slog.Logger
	ctx   context.Context
}

func (s *Server) ListenAndServe(ctx context.Context) error {
	s.conf.SetDefaults()
	if err := s.conf.Validate(); err != nil {
		return fmt.Errorf("validate server config: %w", err)
	}

	ln, err := quic.ListenAddr(s.conf.Addr, s.conf.TLS, s.conf.QUIC)
	if err != nil {
		return fmt.Errorf("listen quic: %w", err)
	}
	defer ln.Close()

	s.l.Info("lockhub server started")
	defer s.l.Info("lockub server stopped")

	for {
		select {
		case <-ctx.Done():

			return nil
		default:
			conn, err := ln.Accept(ctx)
			if err != nil {
				s.l.Error("accept", err)
				continue
			}

			go func() {
				session, err := s.handleConnect(conn)
				if err != nil {
					s.l.Error("handle connect", err)
					return
				}
				s.l.Debug("client connected", "client_id", session.ClientID)

				if err := s.handleConn(session, conn); err != nil {
					s.l.Error(fmt.Errorf("handle conn: %w", err).Error())
				}
			}()
		}
	}
}

func ListenAndServe(ctx context.Context, conf *ServerConfig, store Storer,
	logger *slog.Logger) error {
	srv := &Server{
		conf:  conf,
		store: store,
		l:     logger,
		ctx:   ctx,
	}

	return srv.ListenAndServe(ctx)
}

func (s *Server) handleConn(session *dto.Session, conn quic.Connection) error {
	for {
		select {
		case <-s.ctx.Done():
			return nil
		default:
			str, err := conn.AcceptStream(s.ctx)
			if err != nil {
				return fmt.Errorf("accept stream: %w", err)
			}

			go func() {
				if err := s.handleStream(session, str); err != nil {
					s.l.Error(fmt.Errorf("handle stream: %w", err).Error())
				}
			}()
		}
	}
}

func (s *Server) handleStream(session *dto.Session, stream quic.Stream) error {
	defer stream.Close()
	req, err := protocol.ReadRequest(stream)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if err := s.handleRequest(session, req); err != nil {
		return fmt.Errorf("handle request: %w", err)
	}

	return nil
}

func (s *Server) handleConnect(conn quic.Connection) (*dto.Session, error) {
	resp := &protocol.Response{}
	stream, err := conn.AcceptStream(s.ctx)
	if err != nil {
		return nil, fmt.Errorf("accept stream: %w", err)
	}
	defer func(resp *protocol.Response) {
		if err := protocol.WriteResponse(resp, stream); err != nil {
			s.l.Error(fmt.Errorf("write response: %w", err).Error())
		}
		stream.Close()
	}(resp)

	req, err := protocol.ReadRequest(stream)
	if err != nil {
		return nil, fmt.Errorf("read req: %w", err)
	}

	if req.Cmd != protocol.ConnectCommand {
		return nil, errors.New("invalid connect command")
	}

	connect, err := protocol.NewConnect(req.Payload)
	if err != nil {
		return nil, fmt.Errorf("read connect: %w", err)
	}

	sess, err := s.store.GetOrCreateSession(connect.ClientID,
		s.conf.KeepaliveInterval+s.conf.SessionRetentionPeriod)
	if err != nil {
		return nil, fmt.Errorf("get session: %w", err)
	}

	resp.Success = true
	return sess, nil
}
