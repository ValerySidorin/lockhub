package lockhub

import (
	"crypto/tls"
	"errors"

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

	return nil
}

func (c *ServerConfig) SetDefaults() {
	c.Service.SetDefaults()
}
