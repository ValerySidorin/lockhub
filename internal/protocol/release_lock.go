package protocol

import "fmt"

type ReleaseLock struct {
	Name string
}

func NewReleaseLock(payload []byte) (ReleaseLock, error) {
	var res ReleaseLock

	name, _, err := DecodeString(payload)
	if err != nil {
		return res, fmt.Errorf("decode string: %w", err)
	}

	res.Name = name
	return res, nil
}

func (r ReleaseLock) Bytes() []byte {
	payload := EncodeString(r.Name)
	return payload
}
