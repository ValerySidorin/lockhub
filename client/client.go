package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

type Byter interface {
	Bytes() []byte
}

type Client struct {
	conf    ClientConfig
	conn    quic.Connection
	isReady bool
}

func NewClient(ctx context.Context, conf ClientConfig) (*Client, error) {
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

	return &Client{
		conf: conf,
		conn: conn,
	}, nil
}

func (c *Client) TryAcquireLock(name string) error {
	return c.TryAcquireLockVersion(name, 0)
}

func (c *Client) TryAcquireLockVersion(name string, version uint64) error {
	req := protocol.Request{
		Cmd:     protocol.TryAcquireLockCommand,
		Payload: protocol.TryAcquireLock{Name: name, Version: version}.Bytes(),
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	if !resp.Success {
		return errors.New("failed to acquire lock")
	}

	return nil
}

func (c *Client) ReleaseLock(name string) error {
	req := protocol.Request{
		Cmd:     protocol.ReleaseLockCommand,
		Payload: protocol.ReleaseLock{Name: name}.Bytes(),
	}

	resp, err := c.sendRequest(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}

	if !resp.Success {
		return errors.New("failed to release lock")
	}

	return nil
}

func (c *Client) sendRequest(req protocol.Request) (protocol.Response, error) {
	stream, err := c.conn.OpenStream()
	if err != nil {
		return protocol.Response{}, fmt.Errorf("open stream: %w", err)
	}
	defer stream.Close()

	resp, err := roundTrip(req, stream)
	if err != nil {
		return resp, fmt.Errorf("round trip: %w", err)
	}

	return resp, nil
}

func roundTrip(req protocol.Request, stream quic.Stream) (protocol.Response, error) {
	if err := protocol.WriteRequest(req, stream); err != nil {
		return protocol.Response{}, fmt.Errorf("write request: %w", err)
	}
	stream.Close()

	resp, err := protocol.ReadResponse(stream)
	if err != nil {
		return resp, fmt.Errorf("read response: %w", err)
	}

	return resp, nil
}
