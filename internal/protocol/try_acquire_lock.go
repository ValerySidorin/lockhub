package protocol

import (
	"fmt"
)

type TryAcquireLock struct {
	Name    string
	Version uint64
}

func NewTryAcquireLock(payload []byte) (TryAcquireLock, error) {
	var res TryAcquireLock

	name, payload, err := DecodeString(payload)
	if err != nil {
		return res, fmt.Errorf("decode name: %w", err)
	}

	version, _, err := DecodeUint64(payload)
	if err != nil {
		return res, fmt.Errorf("decode version: %w", err)
	}

	res.Name = name
	res.Version = version

	return res, nil
}

func (r TryAcquireLock) Bytes() []byte {
	payload := EncodeString(r.Name)
	payload = append(payload, EncodeUint64(r.Version)...)
	return payload
}
