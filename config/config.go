package config

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ValerySidorin/lockhub"
	"github.com/ValerySidorin/lockhub/internal/raft"
	"github.com/quic-go/quic-go"
)

type RaftConfig struct {
	BindAddr             string
	JoinAddr             string
	NodeID               string
	Dir                  string
	SnapshotsRetainCount int
	Timeout              time.Duration
}

type TLSConfig struct {
	ClientCertsDir    string
	ServerCertPEMPath string
	ServerKeyPEMPath  string
	MTLSEnabled       bool
	NextProtos        string
}

type QUICConfig struct {
	MaxIncomingStreams   int64
	KeepAlivePeriod      time.Duration
	HandshakeIdleTimeout time.Duration
	MaxIdleTimeout       time.Duration
}

type LogConfig struct {
	Level string
}

type Config struct {
	Addr                     string
	SessionKeepAliveInterval time.Duration
	SessionRetentionDuration time.Duration
	LockRetentionDuration    time.Duration
	Raft                     RaftConfig
	TLS                      TLSConfig
	QUIC                     QUICConfig
	Log                      LogConfig
}

func (c *Config) RegisterFlags(f *flag.FlagSet) {
	f.StringVar(&c.Addr, "addr", "", `Lockhub QUIC server address`)
	f.DurationVar(&c.SessionKeepAliveInterval, "session-keepalive-interval", 0, "Lockhub session keepalive interval")
	f.DurationVar(&c.SessionRetentionDuration, "session-retention-duration", 0, "Lockhub session retention duration")
	f.DurationVar(&c.LockRetentionDuration, "lock-retention-duration", 0, "Lockhub lock retention duration")
	c.Raft.RegisterFlags("raft", f)
	c.TLS.RegisterFlags("tls", f)
	c.QUIC.RegisterFlags("quic", f)
	c.Log.RegisterFlags("log", f)
}

func (c *RaftConfig) RegisterFlags(prefix string, f *flag.FlagSet) {
	f.StringVar(&c.BindAddr, prefixName(prefix, "bind-addr"), "", `Raft bind address`)
	f.StringVar(&c.JoinAddr, prefixName(prefix, "join-addr"), "", `Raft join address`)
	f.StringVar(&c.NodeID, prefixName(prefix, "node-id"), "", `Raft node ID`)
	f.StringVar(&c.Dir, prefixName(prefix, "dir"), "", `Raft dir`)
	f.IntVar(&c.SnapshotsRetainCount, prefixName(prefix, "snapshots-retain-count"), 2, `Raft snapshots retain count`)
	f.DurationVar(&c.Timeout, prefixName(prefix, "timeout"), 0, `Raft timeout`)
}

func (c *TLSConfig) RegisterFlags(prefix string, f *flag.FlagSet) {
	f.StringVar(&c.ClientCertsDir, prefixName(prefix, "client-certs-dir"), "", `TLS client certs dir`)
	f.StringVar(&c.ServerCertPEMPath, prefixName(prefix, "server-cert-path"), "", `TLS server cert path`)
	f.StringVar(&c.ServerKeyPEMPath, prefixName(prefix, "server-key-path"), "", `TLS server key path`)
	f.BoolVar(&c.MTLSEnabled, prefixName(prefix, "mtls-enabled"), false, `TLS mTLS enabled flag`)
	f.StringVar(&c.NextProtos, prefixName(prefix, "next-protos"), "", `TLS next protos, comma-separated`)
}

func (c *QUICConfig) RegisterFlags(prefix string, f *flag.FlagSet) {
	f.Int64Var(&c.MaxIncomingStreams, prefixName(prefix, "max-incoming-streams"), 0, `QUIC max incoming streams`)
	f.DurationVar(&c.KeepAlivePeriod, prefixName(prefix, "keepalive-period"), 0, `QUIC keepalive period`)
	f.DurationVar(&c.HandshakeIdleTimeout, prefixName(prefix, "handshake-idle-timeout"), 0, `QUIC handshake idle timeout`)
	f.DurationVar(&c.MaxIdleTimeout, prefixName(prefix, "max-idle-timeout"), 0, `QUIC max idle timeout`)
}

func (c *LogConfig) RegisterFlags(prefix string, f *flag.FlagSet) {
	f.StringVar(&c.Level, prefixName(prefix, "level"), "info", `Log level`)
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
		RaftService: raft.RaftServiceConfig{
			SessionKeepAliveInterval: c.SessionKeepAliveInterval,
			SessionRetentionDuration: c.SessionRetentionDuration,
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

	caCertPool := x509.NewCertPool()

	if c.ClientCertsDir != "" {
		clientCAs, err := os.ReadDir(c.ClientCertsDir)
		if err != nil {
			return nil, fmt.Errorf("read client CAs dir: %w", err)
		}

		for _, certEntry := range clientCAs {
			if !certEntry.IsDir() {
				cert, err := os.ReadFile(filepath.Join(c.ClientCertsDir, certEntry.Name()))
				if err != nil {
					return nil, fmt.Errorf("read client CA: %w", err)
				}
				caCertPool.AppendCertsFromPEM(cert)
			}
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

	nextProtos := strings.Split(c.NextProtos, ",")

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caCertPool,
		ClientAuth:   clientAuth,
		NextProtos:   nextProtos,
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

func prefixName(prefix, name string) string {
	return prefix + "." + name
}
