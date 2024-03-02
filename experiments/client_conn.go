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
	"github.com/ValerySidorin/lockhub/server"
)

const addr = "localhost:4242"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() { log.Fatal(server.ListenAndServe(ctx, addr, generateTLSConfig(), nil)) }()

	_, err := client.NewClient(ctx, addr, generateTLSConfig(), nil)
	if err != nil {
		log.Fatal(err)
	}

	time.Sleep(60 * time.Second)
	cancel()
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
		NextProtos:         []string{"quic-lockhub"},
	}
}
