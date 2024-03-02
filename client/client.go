package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

type Client struct {
}

func NewClient(ctx context.Context, addr string, tlsConf *tls.Config, quicConf *quic.Config) (*Client, error) {
	conn, err := quic.DialAddr(ctx, addr, tlsConf, quicConf)
	if err != nil {
		return nil, fmt.Errorf("dial quic: %w", err)
	}

	go func() {
		for {
			req := protocol.KeepaliveRequest()

			fmt.Println("before open stream")
			str, err := conn.OpenStream()
			if err != nil {
				log.Println(err)
				return
			}
			defer str.Close()

			fmt.Println("after open stream")
			if err := protocol.Write(req, str); err != nil {
				log.Println(err)
			}
			str.Close()

			fmt.Println("write keepalive")
			_, err = io.ReadAll(str)
			if err != nil {
				log.Println(err)
			}
		}
	}()

	return &Client{}, nil
}
