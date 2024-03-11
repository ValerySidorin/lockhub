package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/ValerySidorin/lockhub"
	"github.com/ValerySidorin/lockhub/config"
	"github.com/hashicorp/raft"
	"github.com/jessevdk/go-flags"
	raftfastlog "github.com/tidwall/raft-fastlog"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	var conf config.Config

	flags.Parse(&conf)

	serverConf, err := conf.Parse()
	if err != nil {
		logger.Error(fmt.Errorf("parse server conf: %w", err).Error())
		os.Exit(1)
	}

	snapshots, err := raft.NewFileSnapshotStore(
		conf.Raft.Dir, conf.Raft.SnapshotsRetainCount, os.Stdout)
	if err != nil {
		logger.Error(fmt.Errorf("new raft snapshots store: %w", err).Error())
		os.Exit(1)
	}

	raftDB, err := raftfastlog.NewFastLogStore(
		filepath.Join(conf.Raft.Dir, "raft.db"), raftfastlog.High, os.Stdout)
	if err != nil {
		logger.Error(fmt.Errorf("new raft db: %w", err).Error())
		os.Exit(1)
	}

	server := lockhub.NewServer(serverConf,
		lockhub.WithRaftLogStore(raftDB),
		lockhub.WithRaftStableStore(raftDB),
		lockhub.WithRaftSnapshotStore(snapshots),
		lockhub.WithLogger(logger))

	if err := server.ListenAndServe(ctx); err != nil {
		logger.Error(fmt.Errorf("listen and serve: %w", err).Error())
	}
}
