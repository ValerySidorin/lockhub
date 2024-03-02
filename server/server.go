package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"time"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

type Server struct {
}

func ListenAndServe(ctx context.Context, addr string,
	tlsConf *tls.Config, quicConf *quic.Config) error {
	ln, err := quic.ListenAddr(addr, tlsConf, quicConf)
	if err != nil {
		return fmt.Errorf("listen quic: %w", err)
	}
	defer ln.Close()

	for {
		conn, err := ln.Accept(ctx)
		if err != nil {
			log.Println(err)
			return nil
		}

		go handleConn(ctx, conn)
	}
}

func handleConn(ctx context.Context, conn quic.Connection) {
	for {
		fmt.Println("before accept stream")
		str, err := conn.AcceptStream(ctx)
		if err != nil {
			log.Println(err)
			return
		}
		fmt.Println("accept stream")

		go func() {
			fmt.Println("before read")
			req, err := protocol.Read(str)
			if err != nil {
				log.Println(err)
				return
			}
			fmt.Println("read req")

			switch req.Cmd {
			case protocol.KeepaliveCommand:
				fmt.Println("received keepalive")
				time.Sleep(10 * time.Second)
				str.Close()
			}
		}()
	}
}
