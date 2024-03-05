package protocol

import (
	"encoding/binary"
	"errors"
)

func DecodeCmd(p []byte) (Command, []byte, error) {
	if len(p) < 1 {
		return UnknownCommand, p, errors.New("invalid buf")
	}
	cmd := Command(p[0])
	if cmd == UnknownCommand {
		return cmd, p, errors.New("unknown command")
	}

	return cmd, p[1:], nil
}

func EncodeCmd(c Command) []byte {
	res := make([]byte, 1)
	res = append(res, []byte{byte(c)}...)
	return res
}

func EncodeString(val string) []byte {
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, uint64(len(val)))
	res = append(res, []byte(val)...)
	return res
}

func DecodeString(p []byte) (string, []byte, error) {
	if len(p) < 8 {
		return "", p, errors.New("invalid buf: len")
	}
	lenBytes := p[:8]
	valLen := binary.LittleEndian.Uint64(lenBytes)
	if len(p) < int(valLen+8) {
		return "", p, errors.New("invalid buf: val")
	}
	return string(p[8 : valLen+8]), p[valLen+8:], nil
}

func EncodeUint64(val uint64) []byte {
	res := make([]byte, 8)
	binary.LittleEndian.PutUint64(res, val)
	return res
}

func DecodeUint64(p []byte) (uint64, []byte, error) {
	if len(p) < 8 {
		return 0, p, errors.New("invalid buf: len")
	}
	uint64Bytes := p[:8]
	res := binary.LittleEndian.Uint64(uint64Bytes)
	return res, p[8:], nil
}
