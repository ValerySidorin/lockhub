package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Command byte

const (
	KeepaliveCommand Command = iota + 1
	SomeOtherCommand
)

type Request struct {
	Cmd     Command
	Payload []byte
}

func (r *Request) Bytes() []byte {
	payloadLenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(payloadLenBuf, uint64(len(r.Payload)))
	res := []byte{byte(r.Cmd)}
	res = append(res, payloadLenBuf...)
	res = append(res, r.Payload...)
	return res
}

func KeepaliveRequest() Request {
	return Request{
		Cmd: KeepaliveCommand,
	}
}

func Read(r io.Reader) (Request, error) {
	var req Request

	data, err := io.ReadAll(r)
	if err != nil {
		return req, fmt.Errorf("read request: %w", err)
	}
	if len(data) == 0 {
		return req, errors.New("empty request")
	}

	// Read cmd
	req.Cmd = Command(data[0])

	// Read payload
	payloadLenBytes := data[1:9]
	payloadLen := binary.LittleEndian.Uint64(payloadLenBytes)
	if payloadLen > 0 {
		req.Payload = data[9:payloadLen]
	}

	return req, nil
}

func Write(req Request, w io.Writer) error {
	switch req.Cmd {
	case KeepaliveCommand:
		if _, err := w.Write(req.Bytes()); err != nil {
			return fmt.Errorf("write keepalive cmd: %w", err)
		}
	default:
		return errors.New("invalid command")
	}

	return nil
}
