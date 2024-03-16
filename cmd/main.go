package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/ValerySidorin/lockhub"
	"github.com/ValerySidorin/lockhub/config"
	"github.com/hashicorp/raft"
	raftfastlog "github.com/tidwall/raft-fastlog"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	var conf config.Config

	conf.RegisterFlags(flag.CommandLine)

	flag.CommandLine.VisitAll(func(f *flag.Flag) {
		env := strings.ToUpper(f.Name)
		env = strings.Replace(env, ".", "_", -1)
		env = strings.Replace(env, "-", "_", -1)
		env = "LOCKHUB_" + env

		val := os.Getenv(env)
		if val != "" {
			flag.CommandLine.Lookup(f.Name).Value.Set(val)
		}
	})

	flag.Parse()

	logLevel := parseLogLevel(conf.Log.Level)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: logLevel,
	}))
	logger.Info("log level set to: " + logLevel.String())

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

func parseLogLevel(name string) slog.Level {
	switch strings.ToUpper(name) {
	case "DEBUG":
		return slog.LevelDebug
	case "INFO":
		return slog.LevelInfo
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
