package client

import (
	"crypto/tls"
	"errors"

	"github.com/quic-go/quic-go"
)

type ClientConfig struct {
	ClientID string

	Addr string
	TLS  *tls.Config
	QUIC *quic.Config
}

func (c *ClientConfig) Validate() error {
	if c.Addr == "" {
		return errors.New("addr not specified")
	}
	if c.ClientID == "" {
		return errors.New("client ID not specified")
	}

	return nil
}
