package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/discord"
)

const (
	defaultRoundTimer         = 4*time.Hour + 10*time.Minute
	defaultStateRetentionDays = 7
	defaultNotifyRetryWindowH = 24
)

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	ctx := context.Background()
	if err := run(ctx, *configPath); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context, confPath string) error {
	conf, err := config.New(confPath)
	if err != nil {
		return err
	}

	var discordToken string
	var ok bool
	if discordToken, ok = os.LookupEnv("DISCORD_TOKEN"); !ok {
		return fmt.Errorf("DISCORD_TOKEN env not set")
	}

	discordSession, err := discordgo.New(fmt.Sprintf("Bot %s", discordToken))
	if err != nil {
		return err
	}

	level := slog.LevelInfo
	if debug := os.Getenv("DEBUG"); ok && debug == "true" {
		level = slog.LevelDebug
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))

	stateStorePath := conf.StateStorePath
	if stateStorePath == "" {
		stateStorePath = "state.db"
	}

	store, err := internal.NewBboltStateStore(stateStorePath)
	if err != nil {
		return err
	}
	defer store.Close()

	retentionDays := conf.StateRetentionDays
	if retentionDays == 0 {
		retentionDays = defaultStateRetentionDays
	}

	retryWindowHours := conf.NotifyRetryWindowHours
	if retryWindowHours == 0 {
		retryWindowHours = defaultNotifyRetryWindowH
	}

	w := internal.NewWatcher(logger)

	type serverEntry struct {
		handler   *internal.Handler
		processor *internal.RoundProcessor
		paths     []string
	}
	entries := make([]serverEntry, 0, len(conf.Servers))

	for name, server := range conf.Servers {
		var uploader internal.Uploader

		if server.Upload.HTTPS != nil {
			uploader = internal.NewHTTPSUploader(*server.Upload.HTTPS)
		} else if server.Upload.SCP != nil {
			uploader, err = internal.NewSCPUploader(*server.Upload.SCP)
			if err != nil {
				return err
			}
		} else {
			return errors.New("no upload method configured for server " + name)
		}

		discordClient, err := discord.New(discordSession, server.Discord.ChannelID, server.Discord.URLS)
		if err != nil {
			return err
		}

		processor := internal.NewRoundProcessor(logger, name, uploader, store, discordClient, server.Artifacts, server.Discord.URLS)

		roundTimeout := server.RoundTimeout
		if roundTimeout == 0 {
			roundTimeout = defaultRoundTimer
		}

		handler, err := internal.NewHandler(logger, processor, server.Artifacts, roundTimeout)
		if err != nil {
			return err
		}

		paths := make([]string, 0, len(server.Artifacts))
		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Location)
		}

		entries = append(entries, serverEntry{handler, processor, paths})
	}

	blockCh := make(chan struct{})
	discordSession.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		slog.Info("Bot is up and running")
		blockCh <- struct{}{}
	})

	go func() {
		if err := discordSession.Open(); err != nil {
			slog.Error("failed to start bot", "error", err)
		}
	}()
	defer discordSession.Close()

	<-blockCh

	retentionCutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
	replaySince := time.Now().Add(-time.Duration(retryWindowHours) * time.Hour)

	if err := store.PurgeCompleted(retentionCutoff); err != nil {
		slog.Error("failed to purge old state records", "error", err)
	}

	for _, e := range entries {
		defer e.handler.Close()
		w.Register(e.paths, e.handler)

		if err := e.processor.ReplayUnnotified(ctx, replaySince); err != nil {
			slog.Error("failed to replay unnotified rounds", "error", err)
		}

		if err := e.processor.UploadOldFiles(ctx); err != nil {
			slog.Error("failed to upload old files", "error", err)
		}
	}

	return w.Watch(ctx)
}
