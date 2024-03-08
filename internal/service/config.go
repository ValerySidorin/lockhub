package service

import (
	"time"
)

const (
	// Raft defaults
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second

	// Lockhub defaults
	defaultKeepaliveInterval      = 12 * time.Second
	defaultSessionRetentionPeriod = 5 * time.Minute
)

type ServiceConfig struct {
	Raft                     RaftConfig `mapstructure:"raft"`
	KeepaliveInterval        time.Duration
	SessionRetentionDuration time.Duration
	LockRetentionDuration    time.Duration
}

type RaftConfig struct {
	Dir    string `mapstructure:"dir"`
	Bind   string `mapstructure:"bind"`
	Join   string `mapstructure:"join"`
	NodeID string `mapstructure:"node_id"`
}

func (c *ServiceConfig) SetDefaults() {
	if c.KeepaliveInterval == 0 {
		c.KeepaliveInterval = defaultKeepaliveInterval
	}
	if c.SessionRetentionDuration == 0 {
		c.SessionRetentionDuration = defaultSessionRetentionPeriod
	}
}
