package protocol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type OpCode byte

const (
	UnknownOpCode OpCode = iota
	ConnectOpCode
	KeepaliveOpCode
	TryAcquireLockOpCode
	ReleaseLockOpCode
)

type RespCode byte

const (
	UnknownRespCode = iota
	OKRespCode
	NotMasterRespCode
	UnhandledErrRespCode
)

type Request struct {
	Cmd     OpCode
	Payload []byte
}

type Response struct {
	Success bool
}

func (r *Request) Bytes() []byte {
	res := []byte{byte(r.Cmd)}

	if r.Payload == nil {
		return res
	}

	payloadLenBuf := make([]byte, 8)
	binary.LittleEndian.PutUint64(payloadLenBuf, uint64(len(r.Payload)))
	res = append(res, payloadLenBuf...)
	res = append(res, r.Payload...)
	return res
}

func ReadRequest(r io.Reader) (Request, error) {
	var req Request

	data, err := io.ReadAll(r)
	if err != nil {
		return req, fmt.Errorf("read request: %w", err)
	}
	if len(data) == 0 {
		return req, errors.New("empty request")
	}

	// Read cmd
	req.Cmd = OpCode(data[0])
	if len(data) == 1 {
		return req, nil
	}

	// Read payload
	payloadLenBytes := data[1:9]
	payloadLen := binary.LittleEndian.Uint64(payloadLenBytes)
	if payloadLen > 0 {
		req.Payload = data[9 : payloadLen+9]
	}

	return req, nil
}

func WriteRequest(req Request, w io.Writer) error {
	if _, err := w.Write(req.Bytes()); err != nil {
		return fmt.Errorf("write to stream: %w", err)
	}

	return nil
}

func (r *Response) Bytes() []byte {
	var boolByte byte
	if r.Success {
		boolByte = 1
	} else {
		boolByte = 0
	}
	return []byte{byte(boolByte)}
}

func ReadResponse(r io.Reader) (Response, error) {
	var resp Response

	data, err := io.ReadAll(r)
	if err != nil {
		return resp, fmt.Errorf("read bytes: %w", err)
	}

	if len(data) == 0 {
		return resp, errors.New("empty response")
	}

	resp.Success = data[0]%2 == 1
	return resp, nil
}

func WriteResponse(resp *Response, w io.Writer) error {
	if _, err := w.Write(resp.Bytes()); err != nil {
		return fmt.Errorf("write resp: %w", err)
	}

	return nil
}
