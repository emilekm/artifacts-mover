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

	"database/sql"

	"github.com/bwmarrin/discordgo"
	"github.com/emilekm/artifacts-mover/internal/config"
	"github.com/emilekm/artifacts-mover/internal/db"
	"github.com/emilekm/artifacts-mover/internal/ingest"
	"github.com/emilekm/artifacts-mover/internal/notify"
	"github.com/emilekm/artifacts-mover/internal/types"
	"github.com/emilekm/artifacts-mover/internal/upload"
	"golang.org/x/sync/errgroup"

	_ "modernc.org/sqlite"

	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riversqlite"
	"github.com/riverqueue/river/rivermigrate"
)

const (
	defaultRoundTimeout   = 4*time.Hour + 10*time.Minute
	defaultStateStorePath = "state.db"
	purgeInterval         = 6 * time.Hour
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

	dbPool, err := sql.Open("sqlite", "file:./river.sqlite3")
	if err != nil {
		return err
	}
	defer dbPool.Close()

	dbPool.SetMaxOpenConns(1)

	db, err := db.NewDB(dbPool)
	if err != nil {
		return err
	}

	migrator, err := rivermigrate.New(riversqlite.New(dbPool), nil)
	if err != nil {
		return err
	}

	_, err = migrator.Migrate(ctx, rivermigrate.DirectionUp, nil)
	if err != nil {
		return err
	}

	uploads := upload.NewWorker(logger, db)
	notifications := notify.NewWorker(logger, db)

	workers := river.NewWorkers()
	river.AddWorker(workers, uploads)
	river.AddWorker(workers, notifications)

	riverClient, err := river.NewClient(riversqlite.New(dbPool), &river.Config{
		Queues: map[string]river.QueueConfig{
			river.QueueDefault: {MaxWorkers: 100},
		},
		Logger:  logger,
		Workers: workers,
	})
	if err != nil {
		return err
	}

	watcher := ingest.NewWatcher(logger)

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

		handler := ingest.NewHandler(ctx, logger, db, riverClient, server.Artifacts, roundTimeout, name)
		defer handler.Close()

		scanners = append(scanners, ingest.NewScanner(logger, db, riverClient, handler, server.Artifacts, name))

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
	group.Go(func() error { return riverClient.Start(ctx) })

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
