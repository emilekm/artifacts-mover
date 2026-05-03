package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"time"

	abase "github.com/Alliance-Community/bots-base"
	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/discord"
)

const (
	defaultRoundTimer = 4*time.Hour + 10*time.Minute // max round time (4h) + max pre-round timer (5min) + leisure (5min)
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

	discordConfig, err := abase.GetConfigFromEnv("MOVER")
	if err != nil {
		return err
	}

	logger := abase.NewLogger(discordConfig)
	slog.SetDefault(logger)

	bot, err := abase.NewBot(discordConfig, 0, logger)
	if err != nil {
		return err
	}

	// TODO: open StateStore (stateStorePath from config)

	w := internal.NewWatcher()

	handlers := make([]*internal.Handler, 0)

	for name, server := range conf.Servers {
		// TODO: wire RoundProcessor with real uploader + StateStore once
		// httpsUploader/scpUploader implement the single-file Uploader interface.
		_ = name

		discordClient, err := discord.New(bot.Session(), server.Discord.ChannelID, server.Discord.URLS)
		if err != nil {
			return err
		}
		_ = discordClient

		roundTimeout := server.RoundTimeout
		if roundTimeout == 0 {
			roundTimeout = defaultRoundTimer
		}

		handler, err := internal.NewHandler(
			func(internal.Round) { /* TODO: processor.Process(ctx, round) */ },
			server.Artifacts,
			roundTimeout,
		)
		if err != nil {
			return err
		}

		handlers = append(handlers, handler)

		paths := make([]string, 0, len(server.Artifacts))
		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Location)
		}

		defer handler.Close()

		w.Register(paths, handler)
	}

	blockCh := make(chan struct{})

	bot.Session().AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		logger.Info("Bot is up and running")
		blockCh <- struct{}{}
	})

	go func() {
		if err := bot.Start(); err != nil {
			logger.Error("failed to start bot", "error", err)
		}
	}()
	defer bot.Stop()

	<-blockCh

	for _, handler := range handlers {
		if err := handler.UploadOldFiles(); err != nil {
			logger.Error("failed to upload old files", "error", err)
		}
	}

	return w.Watch(ctx)
}
