package protocol

type Keepalive struct {
	ClientID string
}

func NewKeepalive(payload []byte) (Keepalive, error) {
	clientID, _, err := DecodeString(payload)
	return Keepalive{ClientID: clientID}, err
}

func (k Keepalive) Bytes() []byte {
	return EncodeString(k.ClientID)
}
