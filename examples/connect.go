package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log"
	"math/big"
	"time"

	"github.com/ValerySidorin/lockhub/client"
)

const addr = "localhost:13796"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// serverConf := &lockhub.ServerConfig{
	// 	Addr:                   addr,
	// 	TLS:                    generateTLSConfig(),
	// 	KeepaliveInterval:      12 * time.Second,
	// 	SessionRetentionPeriod: 5 * time.Minute,
	// }
	// logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
	// 	Level: slog.LevelDebug,
	// }))
	// store := lockhub.NewInmemStore(logger)
	// go func() { log.Fatal(lockhub.ListenAndServe(ctx, serverConf, store, logger)) }()

	clientConf := client.ClientConfig{
		Addr:     addr,
		ClientID: "1234567",
		TLS:      &tls.Config{InsecureSkipVerify: true, NextProtos: []string{"lockhub"}},
	}
	c, err := client.NewClient(ctx, clientConf)
	if err != nil {
		log.Fatal(err)
	}

	if err := c.TryAcquireLockVersion("test", 123); err != nil {
		log.Println(err, " 1234567")
	}

	time.Sleep(1 * time.Second)

	// if err := c.ReleaseLock("test"); err != nil {
	// 	log.Fatal(err)
	// }

	time.Sleep(100 * time.Second)
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
