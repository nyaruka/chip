package main

import (
	ulog "log"
	"log/slog"
	"os"
	"os/signal"
	goruntime "runtime"
	"syscall"

	"github.com/nyaruka/ezconf"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

var (
	// https://goreleaser.com/cookbooks/using-main.version
	version = "dev"
	date    = "unknown"
)

func main() {
	config := runtime.NewDefaultConfig()
	config.Version = version
	loader := ezconf.NewLoader(
		config,
		"chatserver", "Temba Chat - webchat server",
		[]string{"config.toml"},
	)
	loader.MustLoad()

	var level slog.Level
	err := level.UnmarshalText([]byte(config.LogLevel))
	if err != nil {
		ulog.Fatalf("invalid log level %s", level)
		os.Exit(1)
	}

	// configure our logger
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	slog.SetDefault(slog.New(logHandler))

	logger := slog.With("comp", "main")
	logger.Info("starting chatserver", "version", version, "released", date)

	cs := webchat.NewServer(config)
	if err := cs.Start(); err != nil {
		logger.Error("unable to start server", "error", err)
		os.Exit(1)
	}

	handleSignals(cs) // handle our signals
}

// handleSignals takes care of trapping quit, interrupt or terminate signals and doing the right thing
func handleSignals(cs webchat.Server) {
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
			cs.Stop()
			return
		}
	}
}
