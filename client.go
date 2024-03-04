package lockhub

import (
	"context"
	"errors"
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

	stream, err := conn.OpenStream()
	if err != nil {
		return nil, fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	req := protocol.Request{
		Cmd:     protocol.ConnectCommand,
		Payload: protocol.Connect{ClientID: conf.ClientID}.Bytes(),
	}
	if err := protocol.WriteRequest(req, stream); err != nil {
		return nil, fmt.Errorf("write connect request: %w", err)
	}
	stream.Close()

	resp, err := protocol.ReadResponse(stream)
	if err != nil {
		return nil, fmt.Errorf("read connect response: %w", err)
	}

	if !resp.Success {
		return nil, errors.New("connect response unsuccessful")
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
				Payload: protocol.Connect{ClientID: conf.ClientID}.Bytes(),
			}

			if err := protocol.WriteRequest(req, str); err != nil {
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

	return &Client{conf: conf}, nil
}
