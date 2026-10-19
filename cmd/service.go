package cmd

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nyaruka/chip/v26/core/courier"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/chip/v26/web"
)

// shutdownTimeout is how long we allow for a graceful shutdown before exiting hard. Past this budget something is
// genuinely wedged, and it's better to exit with a record of why than be killed by the orchestrator, whose stop
// timeout should be set a bit above this so the watchdog fires first.
const shutdownTimeout = 60 * time.Second

// Service starts the chip service, blocks until a termination signal is received, then stops it. The config must
// already be loaded, e.g. with LoadConfig. All logging is sent to the given handler, e.g. LogHandler(), whose level
// is set from the config.
func Service(cfg *runtime.Config, version, date string, logHandler slog.Handler) error {
	cfg.Version = version

	// configure our logger
	logLevel.Set(cfg.LogLevel)
	slog.SetDefault(slog.New(logHandler))

	log := slog.With("comp", "main")
	log.Info("starting chip", "version", version, "released", date)

	rt, err := runtime.NewRuntime(cfg)
	if err != nil {
		return err
	}

	// log what we can and can't reach before we start doing anything with it
	testConnections(rt)

	server := web.NewServer(rt, courier.NewClient(rt))
	if err := server.Start(); err != nil {
		rt.Stop()
		return err
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	log.Info("stopping", "signal", <-ch)

	watchdog := time.AfterFunc(shutdownTimeout, func() {
		log.Error("shutdown timed out, exiting", "timeout", shutdownTimeout)
		os.Exit(1)
	})
	defer watchdog.Stop()

	if err := server.Stop(); err != nil {
		log.Error("error stopping server", "error", err)
	}
	rt.Stop()

	return nil
}
