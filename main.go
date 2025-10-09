package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/emilekm/artifacts-mover/internal"
	"github.com/emilekm/artifacts-mover/internal/config"
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

	failedPath := conf.FailedUploadPath
	if failedPath == "" {
		failedPath = "./failed"
	}

	w := internal.NewWatcher()
	// TODO: implement queue in simple uploaders when needed
	// q := internal.NewQueue()

	for name, server := range conf.Servers {
		svFailedPath := filepath.Join(failedPath, name)
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

		locToType := make(map[string]config.ArtifactType)
		for typ, loc := range server.Artifacts {
			locToType[filepath.Clean(loc.Location)] = typ
		}

		var webhook *internal.DiscordWebhook
		if server.Discord.WebhookURL != "" {
			webhook, err = internal.NewDiscordWebhook(server.Discord.WebhookURL, server.Discord.URLS)
			if err != nil {
				return err
			}
		}

		handler := internal.NewHandler(uploader, locToType, webhook)

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

	return w.Watch(ctx)
}
