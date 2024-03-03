package lockhub

import (
	"context"
	"fmt"
	"log/slog"

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
				if err := s.handleConn(conn); err != nil {
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

func (s *Server) handleConn(conn quic.Connection) error {
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
				if err := s.handleStream(str); err != nil {
					s.l.Error(fmt.Errorf("handle stream: %w", err).Error())
				}
			}()
		}
	}
}

func (s *Server) handleStream(stream quic.Stream) error {
	defer stream.Close()
	req, err := protocol.Read(stream)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if err := s.handleRequest(req); err != nil {
		return fmt.Errorf("handle request: %w", err)
	}

	return nil
}
