package handler

import (
	"errors"
	"time"

	"github.com/ValerySidorin/lockhub/internal/protocol"
	"github.com/quic-go/quic-go"
)

func HandleRequest(req protocol.Request, stream quic.Stream) error {
	switch req.Cmd {
	case protocol.KeepaliveCommand:
		return handleKeepaliveCommand()
	default:
		return errors.New("unknown command")
	}
}

func handleKeepaliveCommand() error {
	time.Sleep(10 * time.Second)
	return nil
}
