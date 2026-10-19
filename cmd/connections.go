package cmd

import (
	"context"
	"log/slog"
	"time"

	"github.com/nyaruka/chip/v26/runtime"
)

// testConnections checks each backing service in turn and logs the outcome. Nothing here is fatal: a service that
// isn't reachable at startup may be reachable by the time it's needed, and the request that needs it reports the
// problem if not.
func testConnections(rt *runtime.Runtime) {
	log := slog.With("comp", "main")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := rt.DB.PingContext(ctx); err != nil {
		log.Error("db not reachable", "error", err)
	} else {
		log.Info("db ok")
	}

	vc := rt.VK.Get()
	if _, err := vc.Do("PING"); err != nil {
		log.Error("valkey not reachable", "error", err)
	} else {
		log.Info("valkey ok")
	}
	vc.Close()

	if err := rt.S3.Test(ctx, rt.Config.S3AttachmentsBucket); err != nil {
		log.Error("attachments bucket not reachable", "error", err)
	} else {
		log.Info("attachments bucket ok")
	}

	if err := rt.Centrifugo.Client.Info(ctx); err != nil {
		log.Error("centrifugo not reachable", "error", err)
	} else {
		log.Info("centrifugo ok")
	}
}
