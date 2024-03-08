package dto

type Session struct {
	ClientID string          `json:"client_id"`
	Locks    map[string]Lock `json:"locks"`
}
