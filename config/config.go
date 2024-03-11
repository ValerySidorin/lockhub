package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ValerySidorin/lockhub"
	"github.com/ValerySidorin/lockhub/internal/service"
	"github.com/quic-go/quic-go"
)

type RaftConfig struct {
	BindAddr             string        `short:"b" long:"bind" description:"Raft bind address" env:"BIND_ADDR"`
	JoinAddr             string        `long:"join" description:"Raft join address" env:"JOIN_ADDR"`
	NodeID               string        `short:"n" long:"node-id" description:"Raft node ID" env:"NODE_ID"`
	Dir                  string        `short:"d" long:"dir" description:"Raft dir" env:"DIR"`
	SnapshotsRetainCount int           `long:"snapshots-retain-count" description:"Raft snapshots retain count" env:"SNAPSHOTS_RETAIN_COUNT"`
	Timeout              time.Duration `long:"timeout" description:"Raft timeout" env:"TIMEOUT"`
}

type TLSConfig struct {
	ClientCertsDir    string   `long:"client-certs-dir" description:"TLS client certs dir" env:"CLIENT_CERTS_DIR"`
	ServerCertPEMPath string   `long:"server-cert-path" description:"TLS server cert path" env:"SERVER_CERT_PATH"`
	ServerKeyPEMPath  string   `long:"server-key-path" description:"TLS server key path" env:"SERVER_KEY_PATH"`
	MTLSEnabled       bool     `short:"m" long:"mtls-enabled" description:"MTLS enable flag" env:"MTLS_ENABLED"`
	NextProtos        []string `long:"next-proto" description:"TLS next proto" env:"NEXT_PROTOS"`
}

type QUICConfig struct {
	MaxIncomingStreams   int64         `long:"max-incoming-streams" description:"QUIC max incoming streams" env:"MAX_INCOMING_STREAMS"`
	KeepAlivePeriod      time.Duration `long:"keepalive-period" description:"QUIC keepalive period" env:"KEEPALIVE_PERIOD"`
	HandshakeIdleTimeout time.Duration `long:"handshake-idle-timeout" description:"QUIC handshake idle timeout" env:"HANDSHAKE_IDLE_TIMEOUT"`
	MaxIdleTimeout       time.Duration `long:"max-idle-timeout" description:"QUIC max idle timeout" env:"MAX_IDLE_TIMEOUT"`
}

type Config struct {
	Addr                     string        `short:"a" long:"addr" description:"Lockhub QUIC server address" env:"ADDR"`
	Raft                     RaftConfig    `group:"raft" namespace:"raft" env-namespace:"RAFT"`
	TLS                      TLSConfig     `group:"tls" namespace:"tls" env-namespace:"TLS"`
	QUIC                     QUICConfig    `group:"quic" namespace:"quic" env-namespace:"QUIC"`
	SessionKeepaliveInterval time.Duration `long:"session-keepalive-interval" description:"Session keepalive interval. Defaults to 12s"`
	SessionRetentionInterval time.Duration `long:"session-retention-duration" description:"Session retention duration. Defaults to 5m"`
	LockRetentionDuration    time.Duration `long:"lock-retention-duration" description:"Lock retention duration. Defaults to 15m"`
}

func (c *Config) Parse() (lockhub.ServerConfig, error) {
	tlsConf, err := c.TLS.Parse()
	if err != nil {
		return lockhub.ServerConfig{}, fmt.Errorf("parse TLS conf: %w", err)
	}

	raftConf, err := c.Raft.Parse()
	if err != nil {
		return lockhub.ServerConfig{}, fmt.Errorf("parse raft conf: %w", err)
	}

	return lockhub.ServerConfig{
		Addr: c.Addr,
		Raft: raftConf,
		TLS:  tlsConf,
		QUIC: c.QUIC.Parse(),
		Service: service.ServiceConfig{
			KeepaliveInterval:        c.SessionKeepaliveInterval,
			SessionRetentionDuration: c.SessionRetentionInterval,
			LockRetentionDuration:    c.LockRetentionDuration,
		},
	}, nil
}

func (c *RaftConfig) Parse() (lockhub.RaftConfig, error) {
	if err := c.Validate(); err != nil {
		return lockhub.RaftConfig{}, fmt.Errorf("validate: %w", err)
	}

	return lockhub.RaftConfig{
		NodeID:   c.NodeID,
		BindAddr: c.BindAddr,
		JoinAddr: c.JoinAddr,
		Timeout:  c.Timeout,
	}, nil
}

func (c *RaftConfig) Validate() error {
	if c.Dir == "" {
		return errors.New("dir not specified")
	}

	if c.SnapshotsRetainCount <= 0 {
		return errors.New("snapshots retain count must be greater than 0")
	}

	return nil
}

func (c *QUICConfig) Parse() *quic.Config {
	return &quic.Config{
		MaxIncomingStreams:   c.MaxIncomingStreams,
		KeepAlivePeriod:      c.KeepAlivePeriod,
		HandshakeIdleTimeout: c.HandshakeIdleTimeout,
		MaxIdleTimeout:       c.MaxIdleTimeout,
	}
}

func (c *TLSConfig) Parse() (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	clientCAs, err := os.ReadDir(c.ClientCertsDir)
	if err != nil {
		return nil, fmt.Errorf("read client CAs dir: %w", err)
	}

	caCertPool := x509.NewCertPool()

	for _, certEntry := range clientCAs {
		if !certEntry.IsDir() {
			cert, err := os.ReadFile(filepath.Join(c.ClientCertsDir, certEntry.Name()))
			if err != nil {
				return nil, fmt.Errorf("read client CA: %w", err)
			}
			caCertPool.AppendCertsFromPEM(cert)
		}
	}

	cert, err := tls.LoadX509KeyPair(c.ServerCertPEMPath, c.ServerKeyPEMPath)
	if err != nil {
		return nil, fmt.Errorf("load x509 key pair: %w", err)
	}

	clientAuth := tls.NoClientCert
	if c.MTLSEnabled {
		clientAuth = tls.RequireAndVerifyClientCert
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   clientAuth,
		NextProtos:   c.NextProtos,
	}, nil
}

func (c *TLSConfig) Validate() error {
	if c.ClientCertsDir == "" && c.MTLSEnabled {
		return errors.New("client certs dir not specified, while mTLS enabled")
	}

	if c.ServerCertPEMPath == "" {
		return errors.New("server cert path not specified")
	}

	if c.ServerKeyPEMPath == "" {
		return errors.New("server key path not specified")
	}

	return nil
}
