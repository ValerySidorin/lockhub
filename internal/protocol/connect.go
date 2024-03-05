package protocol

type Connect struct {
	ClientID string
}

func NewConnect(payload []byte) (Connect, error) {
	clientID, _, err := DecodeString(payload)
	return Connect{ClientID: clientID}, err
}

func (r Connect) Bytes() []byte {
	return EncodeString(r.ClientID)
}
