package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	abase "github.com/Alliance-Community/bots-base"
	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
)

const (
	defaultRoundTimer = 4*time.Hour + 10*time.Minute // max round time (4h) + pre-round timer (5min) + leisure (5min)
)

var configPath = flag.String("config", "config.yaml", "path to config file")
var logLevel = flag.String("log-level", "info", "log level (debug, info, warn, error)")

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run(ctx context.Context) error {
	flag.Parse()

	level := slog.LevelInfo
	err := level.UnmarshalText([]byte(*logLevel))
	if err != nil {
		return err
	}

	slog.SetLogLoggerLevel(level)

	conf, err := config.New(*configPath)
	if err != nil {
		return err
	}

	discordConfig, err := abase.GetConfigFromEnv("MOVER")
	if err != nil {
		return err
	}

	debug := false
	if _, ok := os.LookupEnv("DEBUG"); ok {
		debug = true
	}

	logger := abase.NewLogger(discordConfig, debug)

	bot, err := abase.NewBot(discordConfig, 0, logger)
	if err != nil {
		return err
	}

	w := internal.NewWatcher()
	// TODO: implement queue in simple uploaders when needed
	// q := internal.NewQueue()

	for name, server := range conf.Servers {
		svFailedPath := filepath.Join(conf.FailedUploadPath, name)
		if err := os.MkdirAll(svFailedPath, 0755); err != nil {
			return err
		}

		var uploader internal.Uploader

		if server.Upload.HTTPS != nil {
			uploader = internal.NewHTTPSUploader(*server.Upload.HTTPS, server.Artifacts)
		} else if server.Upload.SCP != nil {
			uploader, err = internal.NewSCPUploader(*server.Upload.SCP, server.Artifacts)
			if err != nil {
				return err
			}
		} else {
			return errors.New("no upload method configured")
		}

		discordClient, err := internal.NewDiscordClient(bot.Session(), server.Discord.ChannelID, server.Discord.URLS)
		if err != nil {
			return err
		}

		roundTimeout := server.RoundTimeout
		if roundTimeout == 0 {
			roundTimeout = defaultRoundTimer
		}

		handler, err := internal.NewHandler(uploader, discordClient, server.Artifacts, roundTimeout, svFailedPath)
		if err != nil {
			return err
		}

		err = handler.UploadOldFiles()
		if err != nil {
			return err
		}

		paths := make([]string, 0, len(server.Artifacts))

		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Location)
		}

		w.Register(paths, handler)
	}

	go func() {
		if err := bot.Start(); err != nil {
			logger.Error("failed to start bot", "error", err)
			os.Exit(1)
		}
	}()

	defer bot.Stop()

	return w.Watch(ctx)
}
