package lockhub

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/ValerySidorin/lockhub/internal/service"
	"github.com/ValerySidorin/lockhub/store"
	"github.com/quic-go/quic-go"
)

type Server struct {
	conf    ServerConfig
	service service.Service
	l       *slog.Logger
	ctx     context.Context
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

	s.l.Info("lockhub server started", "addr", ln.Addr())
	defer s.l.Info("lockub server stopped")

	for {
		select {
		case <-ctx.Done():

			return nil
		default:
			conn, err := ln.Accept(ctx)
			if err != nil {
				s.l.Error(fmt.Errorf("accept conn: %w", err).Error())
				continue
			}

			go func() {
				clientID, err := s.handleConnect(conn)
				if err != nil {
					s.l.Error(fmt.Errorf("handle connect: %w", err).Error())
					return
				}
				s.l.Debug("client connected", "client_id", clientID)

				if err := s.handleConn(clientID, conn); err != nil {
					s.l.Error(fmt.Errorf("handle conn: %w", err).Error())
				}
			}()
		}
	}
}

func ListenAndServe(ctx context.Context, conf ServerConfig, store store.Storer,
	logger *slog.Logger) error {
	service := service.New(conf.Service, store, logger)
	if err := service.Open(); err != nil {
		return fmt.Errorf("open: %w", err)
	}

	srv := &Server{
		conf:    conf,
		service: service,
		l:       logger,
		ctx:     ctx,
	}

	return srv.ListenAndServe(ctx)
}

func (s *Server) handleConn(clientID string, conn quic.Connection) error {
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
				if err := s.handleStream(clientID, str); err != nil {
					s.l.Error(fmt.Errorf("handle stream: %w", err).Error())
				}
			}()
		}
	}
}

func (s *Server) handleStream(clientID string, stream quic.Stream) error {
	defer stream.Close()
	req, err := protocol.ReadRequest(stream)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	resp := &protocol.Response{}

	if err := s.handleRequest(clientID, req); err != nil {
		if err := protocol.WriteResponse(resp, stream); err != nil {
			return fmt.Errorf("write unsuccessful response: %w", err)
		}

		return nil
	}

	resp.Success = true
	if err := protocol.WriteResponse(resp, stream); err != nil {
		return fmt.Errorf("write successful response: %w", err)
	}

	return nil
}

func (s *Server) handleConnect(conn quic.Connection) (string, error) {
	resp := &protocol.Response{}
	stream, err := conn.AcceptStream(s.ctx)
	if err != nil {
		return "", fmt.Errorf("accept stream: %w", err)
	}
	defer func(resp *protocol.Response) {
		if err := protocol.WriteResponse(resp, stream); err != nil {
			s.l.Error(fmt.Errorf("write response: %w", err).Error())
		}
		stream.Close()
	}(resp)

	req, err := protocol.ReadRequest(stream)
	if err != nil {
		return "", fmt.Errorf("read req: %w", err)
	}

	if req.Cmd != protocol.ConnectOpCode {
		return "", errors.New("invalid connect command")
	}

	connect, err := protocol.NewConnect(req.Payload)
	if err != nil {
		return "", fmt.Errorf("read connect: %w", err)
	}

	if err := s.service.CreateSessionIfNotExists(connect.ClientID); err != nil {
		return "", fmt.Errorf("get or create session: %w", err)
	}

	resp.Success = true
	return connect.ClientID, nil
}
