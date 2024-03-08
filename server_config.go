package lockhub

import (
	"crypto/tls"
	"errors"
	"fmt"

	"github.com/ValerySidorin/lockhub/internal/service"
	"github.com/quic-go/quic-go"
)

type ServerConfig struct {
	Addr    string
	TLS     *tls.Config
	QUIC    *quic.Config
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
}
