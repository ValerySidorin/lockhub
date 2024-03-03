package lockhub

import (
	"context"
	"fmt"
	"io"
	"log"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

type Client struct {
	conf *ClientConfig
}

func NewClient(ctx context.Context, conf *ClientConfig) (*Client, error) {
	if err := conf.Validate(); err != nil {
		return nil, fmt.Errorf("validate client config: %w", err)
	}

	conn, err := quic.DialAddr(ctx, conf.Addr, conf.TLS, conf.QUIC)
	if err != nil {
		return nil, fmt.Errorf("dial quic: %w", err)
	}

	go func() {
		for {
			req := protocol.KeepaliveRequest(conf.ClientID)

			str, err := conn.OpenStream()
			if err != nil {
				log.Println(err)
				return
			}
			defer str.Close()

			if err := protocol.Write(req, str); err != nil {
				log.Println(err)
			}
			str.Close()

			_, err = io.ReadAll(str)
			if err != nil {
				log.Println(err)
			}
		}
	}()

	return &Client{}, nil
}
