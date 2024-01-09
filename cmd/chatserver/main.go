package main

import (
	ulog "log"
	"log/slog"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/nyaruka/tembachat"
)

var (
	// https://goreleaser.com/cookbooks/using-main.version
	version = "dev"
	date    = "unknown"
)

func main() {
	// configure our logger
	logHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})
	slog.SetDefault(slog.New(logHandler))

	logger := slog.With("comp", "main")
	logger.Info("starting chat server", "version", version, "released", date)

	cfg := tembachat.NewDefaultConfig()

	cs := tembachat.NewServer(cfg)
	if err := cs.Start(); err != nil {
		logger.Error("unable to start server", "error", err)
		os.Exit(1)
	}

	handleSignals(cs) // handle our signals
}

// handleSignals takes care of trapping quit, interrupt or terminate signals and doing the right thing
func handleSignals(cs *tembachat.Server) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for {
		sig := <-sigs
		log := slog.With("comp", "main", "signal", sig)

		switch sig {
		case syscall.SIGQUIT:
			buf := make([]byte, 1<<20)
			stacklen := runtime.Stack(buf, true)
			log.Info("received quit signal, dumping stack")
			ulog.Printf("\n%s", buf[:stacklen])
		case syscall.SIGINT, syscall.SIGTERM:
			log.Info("received exit signal, exiting")
			cs.Stop()
			return
		}
	}
}
