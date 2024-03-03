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
			str, err := conn.OpenStream()
			if err != nil {
				log.Println(err)
				return
			}
			defer str.Close()

			req := protocol.Request{
				Cmd:     protocol.KeepaliveCommand,
				Payload: protocol.Keepalive{ClientID: conf.ClientID}.Bytes(),
			}

			if err := protocol.Write(req, str); err != nil {
				log.Println(err)
				return
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
