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
	SessionKeepAliveInterval time.Duration
	SessionRetentionDuration time.Duration
	LockRetentionDuration    time.Duration
}

func (c *ServiceConfig) SetDefaults() {
	if c.SessionKeepAliveInterval == 0 {
		c.SessionKeepAliveInterval = defaultKeepaliveInterval
	}
	if c.SessionRetentionDuration == 0 {
		c.SessionRetentionDuration = defaultSessionRetentionDuration
	}
	if c.LockRetentionDuration == 0 {
		c.LockRetentionDuration = defaultLockRetentionDuration
	}
}

func (c *ServiceConfig) Validate() error {
	if c.SessionKeepAliveInterval+c.SessionRetentionDuration > c.LockRetentionDuration {
		return errors.New("lock retention duration can not be less than or equal to sum of session keepalive interval and session retention duration")
	}

	return nil
}
