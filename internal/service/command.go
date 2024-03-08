package service

import "github.com/ValerySidorin/lockhub/internal/dto"

type opCode byte

const (
	setSessionOpCode opCode = iota
	deleteSessionOpCode

	setLockOpCode
	deleteLockOpCode
)

type command struct {
	OpCode      opCode      `json:"op_code"`
	Session     dto.Session `json:"session,omitempty"`
	ClientID    string      `json:"client_id,omitempty"`
	LockName    string      `json:"lock_name,omitempty"`
	LockVersion uint64      `json:"lock_version,omitempty"`
}
