package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"math/big"
	"os"
	"os/signal"
	"time"

	"github.com/ValerySidorin/lockhub"
	"github.com/ValerySidorin/lockhub/internal/service"
	"github.com/ValerySidorin/lockhub/store"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	serverConf := lockhub.ServerConfig{
		Addr: ":13796",
		TLS:  generateTLSConfig(),
		Service: service.ServiceConfig{
			Raft: service.RaftConfig{
				Dir:    "./raft",
				Bind:   "127.0.0.1:9346",
				NodeID: "node_1",
			},
			KeepaliveInterval:        12 * time.Second,
			SessionRetentionDuration: 10 * time.Second,
			LockRetentionDuration:    10 * time.Second,
		},
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	store := store.NewInmemStore()
	if err := lockhub.ListenAndServe(ctx, serverConf, store, logger); err != nil {
		logger.Error(err.Error())
	}
}

func generateTLSConfig() *tls.Config {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		panic(err)
	}
	template := x509.Certificate{SerialNumber: big.NewInt(1)}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		panic(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		panic(err)
	}
	return &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{tlsCert},
		NextProtos:         []string{"lockhub"},
	}
}
