package main

import (
	ulog "log"
	"log/slog"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	_ "github.com/lib/pq"
	"github.com/nyaruka/tembachat"
	"github.com/nyaruka/tembachat/runtime"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry"
)

var (
	// https://goreleaser.com/cookbooks/using-main.version
	version = "dev"
	date    = "unknown"
)

func main() {
	config := runtime.LoadConfig()
	config.Version = version

	// configure our logger
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: config.LogLevel})
	slog.SetDefault(slog.New(logHandler))

	// if we have a DSN entry, try to initialize it
	if config.SentryDSN != "" {
		err := sentry.Init(sentry.ClientOptions{Dsn: config.SentryDSN, EnableTracing: false})
		if err != nil {
			ulog.Fatalf("error initiating sentry client, error %s, dsn %s", err, config.SentryDSN)
		}

		defer sentry.Flush(2 * time.Second)

		slog.SetDefault(slog.New(
			slogmulti.Fanout(
				logHandler,
				slogsentry.Option{Level: slog.LevelError}.NewSentryHandler(),
			),
		))
	}

	log := slog.With("comp", "main")
	log.Info("starting...", "version", version, "released", date)

	svc := tembachat.NewService(config)
	if err := svc.Start(); err != nil {
		log.Error("unable to start", "error", err)
		os.Exit(1)
	}

	handleSignals(svc) // handle our signals
}

// handleSignals takes care of trapping quit, interrupt or terminate signals and doing the right thing
func handleSignals(svc *tembachat.Service) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for {
		sig := <-sigs
		log := slog.With("comp", "main", "signal", sig)

		switch sig {
		case syscall.SIGQUIT:
			buf := make([]byte, 1<<20)
			stacklen := goruntime.Stack(buf, true)
			log.Info("received quit signal, dumping stack")
			ulog.Printf("\n%s", buf[:stacklen])
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("received exit signal, exiting")
			svc.Stop()
			return
		}
	}
}
