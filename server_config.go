package lockhub

import (
	"crypto/tls"
	"errors"
	"time"

	"github.com/quic-go/quic-go"
)

const (
	DefaultKeepaliveInterval      = 12 * time.Second
	DefaultSessionRetentionPeriod = 5 * time.Minute
)

type ServerConfig struct {
	Addr string
	TLS  *tls.Config
	QUIC *quic.Config

	KeepaliveInterval      time.Duration
	SessionRetentionPeriod time.Duration
}

func (c *ServerConfig) SetDefaults() {
	if c.KeepaliveInterval == 0 {
		c.KeepaliveInterval = DefaultKeepaliveInterval
	}
	if c.SessionRetentionPeriod == 0 {
		c.SessionRetentionPeriod = DefaultSessionRetentionPeriod
	}
}

func (c *ServerConfig) Validate() error {
	if c.Addr == "" {
		return errors.New("addr not specified")
	}

	return nil
}
