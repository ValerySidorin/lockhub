package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"

	"github.com/ValerySidorin/lockhub/internal/handler"
	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

func ListenAndServe(ctx context.Context, addr string,
	tlsConf *tls.Config, quicConf *quic.Config,
	logger *slog.Logger) error {
	ln, err := quic.ListenAddr(addr, tlsConf, quicConf)
	if err != nil {
		return fmt.Errorf("listen quic: %w", err)
	}
	defer ln.Close()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			conn, err := ln.Accept(ctx)
			if err != nil {
				logger.Error("accept", err)
				continue
			}

			go func() {
				if err := handleConn(ctx, conn, logger); err != nil {
					logger.Error("handle conn", err)
				}
			}()
		}
	}
}

func handleConn(ctx context.Context, conn quic.Connection, logger *slog.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			str, err := conn.AcceptStream(ctx)
			if err != nil {
				return fmt.Errorf("accept stream: %w", err)
			}

			go func() {
				if err := handleStream(str); err != nil {
					logger.Error("handle stream", err)
				}
			}()
		}
	}
}

func handleStream(stream quic.Stream) error {
	defer stream.Close()
	req, err := protocol.Read(stream)
	if err != nil {
		return fmt.Errorf("read request: %w", err)
	}

	if err := handler.HandleRequest(req, stream); err != nil {
		return fmt.Errorf("handle request: %w", err)
	}

	return nil
}
