package lockhub

import (
	"crypto/tls"
	"errors"
	"fmt"
	"time"

	"github.com/ValerySidorin/lockhub/internal/service"
	"github.com/quic-go/quic-go"
)

type RaftConfig struct {
	BindAddr string
	JoinAddr string
	NodeID   string
	Timeout  time.Duration
}

type ServerConfig struct {
	Addr    string
	TLS     *tls.Config
	QUIC    *quic.Config
	Raft    RaftConfig
	Service service.ServiceConfig
}

func (c *ServerConfig) Validate() error {
	if c.Addr == "" {
		return errors.New("addr not specified")
	}

	if err := c.Service.Validate(); err != nil {
		return fmt.Errorf("validate service config: %w", err)
	}

	return nil
}

func (c *ServerConfig) SetDefaults() {
	c.Service.SetDefaults()
	c.Raft.SetDefaults()
}

func (c *RaftConfig) Validate() error {
	if c.NodeID == "" {
		return errors.New("node id not specified")
	}

	if c.BindAddr == "" {
		return errors.New("bind addr not specified")
	}

	if c.Timeout == 0 {
		return errors.New("timeout not specified")
	}

	return nil
}

func (c *RaftConfig) SetDefaults() {
	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}
}
