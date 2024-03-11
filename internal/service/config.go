package service

import (
	"errors"
	"time"
)

const (
	// Lockhub defaults
	defaultKeepaliveInterval        = 12 * time.Second
	defaultSessionRetentionDuration = 5 * time.Minute
	defaultLockRetentionDuration    = 15 * time.Minute
)

type ServiceConfig struct {
	KeepaliveInterval        time.Duration
	SessionRetentionDuration time.Duration
	LockRetentionDuration    time.Duration
}

func (c *ServiceConfig) SetDefaults() {
	if c.KeepaliveInterval == 0 {
		c.KeepaliveInterval = defaultKeepaliveInterval
	}
	if c.SessionRetentionDuration == 0 {
		c.SessionRetentionDuration = defaultSessionRetentionDuration
	}
	if c.LockRetentionDuration == 0 {
		c.LockRetentionDuration = defaultLockRetentionDuration
	}
}

func (c *ServiceConfig) Validate() error {
	if c.KeepaliveInterval+c.SessionRetentionDuration > c.LockRetentionDuration {
		return errors.New("lock retention duration can not be less than or equal to sum of session keepalive interval and session retention duration")
	}

	return nil
}
