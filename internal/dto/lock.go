package dto

type Lock struct {
	Name      string `json:"name"`
	ClientID  string `json:"client_id"`
	Version   uint64 `json:"version"`
	UpdatedAt int64  `json:"updated_at"`
}
