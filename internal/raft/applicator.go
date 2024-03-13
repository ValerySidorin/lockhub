package raft

import (
	"encoding/json"
	"time"

	"github.com/hashicorp/raft"
)

type applicatorRequest struct {
	c    command
	resp chan bool
}

type applicator struct {
	raft    *raft.Raft
	timeout time.Duration
	buf     chan applicatorRequest
}

func newApplicator(raft *raft.Raft, timeout time.Duration) *applicator {
	return &applicator{
		raft:    raft,
		timeout: timeout,
		buf:     make(chan applicatorRequest),
	}
}

func (a *applicator) start() {
	for req := range a.buf {

		b, _ := json.Marshal(req.c)
		// TODO: handle apply errors
		a.raft.Apply(b, a.timeout)
	}
}

func (a *applicator) ch() chan<- applicatorRequest {
	return a.buf
}
