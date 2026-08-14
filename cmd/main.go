package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/ingest"
	applog "github.com/emilekm/artifacts-mover/internal/log"
	"github.com/emilekm/artifacts-mover/internal/notify"
	"github.com/emilekm/artifacts-mover/internal/store"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/emilekm/artifacts-mover/internal/upload"
	"golang.org/x/sync/errgroup"
)

const (
	defaultRoundTimeout       = 4*time.Hour + 10*time.Minute
	defaultStateStorePath     = "state.db"
	defaultStateRetentionDays = 7
	defaultNotifyRetryWindow  = time.Hour
	purgeInterval             = 6 * time.Hour
)

var configPath = flag.String("config", "config.yaml", "path to config file")

func main() {
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *configPath); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, confPath string) error {
	logger := newLogger()

	conf, err := config.New(confPath)
	if err != nil {
		return err
	}

	discordToken, ok := os.LookupEnv("DISCORD_TOKEN")
	if !ok {
		return errors.New("DISCORD_TOKEN env not set")
	}

	session, err := discordgo.New(fmt.Sprintf("Bot %s", discordToken))
	if err != nil {
		return err
	}

	stateStorePath := conf.StateStorePath
	if stateStorePath == "" {
		stateStorePath = defaultStateStorePath
	}

	stateStore, err := store.NewBboltStore(stateStorePath)
	if err != nil {
		return err
	}
	defer stateStore.Close()

	retentionDays := conf.StateRetentionDays
	if retentionDays == 0 {
		retentionDays = defaultStateRetentionDays
	}

	notifyRetryWindow := time.Duration(conf.NotifyRetryWindowHours) * time.Hour
	if notifyRetryWindow == 0 {
		notifyRetryWindow = defaultNotifyRetryWindow
	}

	watcher := ingest.NewWatcher(logger)
	uploads := upload.NewWorker(logger, stateStore)
	notifications := notify.NewWorker(logger, stateStore, notifyRetryWindow)

	scanners := make([]*ingest.Scanner, 0, len(conf.Servers))

	for name, server := range conf.Servers {
		uploader, err := newUploader(server)
		if err != nil {
			return fmt.Errorf("server %s: %w", name, err)
		}

		uploads.Register(name, uploader)
		notifications.Register(name, notify.NewDiscordNotifier(logger, session, server.Discord))

		roundTimeout := server.RoundTimeout
		if roundTimeout == 0 {
			roundTimeout = defaultRoundTimeout
		}

		handler := ingest.NewHandler(ctx, logger, stateStore, server.Artifacts, roundTimeout, name)
		defer handler.Close()

		scanners = append(scanners, ingest.NewScanner(logger, stateStore, handler, server.Artifacts, name))

		paths := make([]string, 0, len(server.Artifacts))
		for _, loc := range server.Artifacts {
			paths = append(paths, loc.Location)
		}
		watcher.Register(paths, handler)
	}

	if err := openSession(ctx, logger, session); err != nil {
		return err
	}
	defer session.Close()

	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error { return watcher.Run(ctx, scanAll(scanners)) })
	group.Go(func() error { return uploads.Watch(ctx) })
	group.Go(func() error { return notifications.Watch(ctx) })
	group.Go(func() error { return purgeLoop(ctx, logger, stateStore, retentionDays) })

	return group.Wait()
}

func newLogger() *slog.Logger {
	level := slog.LevelInfo
	if os.Getenv("DEBUG") == "true" {
		level = slog.LevelDebug
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
	}))
}

func newUploader(server *config.Server) (upload.Uploader, error) {
	uploadPaths := make(map[types.ArtifactType]string, len(server.Artifacts))
	for typ, loc := range server.Artifacts {
		uploadPaths[typ] = loc.UploadPath
	}

	switch {
	case server.Upload.HTTPS != nil:
		return upload.NewHTTPSUploader(*server.Upload.HTTPS, uploadPaths), nil
	case server.Upload.SCP != nil:
		return upload.NewSCPUploader(*server.Upload.SCP, uploadPaths)
	default:
		return nil, errors.New("no upload method configured")
	}
}

// openSession connects the bot and waits for the gateway to report ready, so the
// workers don't start sending before the session can serve them.
func openSession(ctx context.Context, logger *slog.Logger, session *discordgo.Session) error {
	ready := make(chan struct{})
	var once sync.Once
	session.AddHandler(func(*discordgo.Session, *discordgo.Ready) {
		once.Do(func() {
			logger.LogAttrs(ctx, slog.LevelInfo, "discord: bot is up and running")
			close(ready)
		})
	})

	if err := session.Open(); err != nil {
		return err
	}

	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		session.Close()
		return ctx.Err()
	}
}

// scanAll folds every server's startup scan into the single scan the watcher
// runs, merging the paths they consumed so none of them get replayed live.
func scanAll(scanners []*ingest.Scanner) func(context.Context) (map[string]struct{}, error) {
	return func(ctx context.Context) (map[string]struct{}, error) {
		consumed := make(map[string]struct{})

		var errs []error
		for _, scanner := range scanners {
			scanned, err := scanner.Scan(ctx)
			maps.Copy(consumed, scanned)
			if err != nil {
				errs = append(errs, err)
			}
		}

		return consumed, errors.Join(errs...)
	}
}

func purgeLoop(ctx context.Context, logger *slog.Logger, stateStore store.Store, retentionDays int) error {
	ticker := time.NewTicker(purgeInterval)
	defer ticker.Stop()

	for {
		cutoff := time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour)
		if err := stateStore.PurgeCompleted(cutoff); err != nil {
			logger.LogAttrs(ctx, slog.LevelError, "store: failed to purge completed rounds", applog.Error(err))
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
