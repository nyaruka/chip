package runtime

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/gomodule/redigo/redis"
)

type Runtime struct {
	DB     *sql.DB
	RP     *redis.Pool
	Config *Config
}

func OpenDBPool(url string, maxOpenConns int) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, fmt.Errorf("unable to open database connection: '%s'", url)
	}

	// configure our pool
	db.SetMaxIdleConns(8)
	db.SetMaxOpenConns(maxOpenConns)
	db.SetConnMaxLifetime(time.Minute * 30)

	// ping database...
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	err = db.PingContext(ctx)
	cancel()

	return db, err
}

// WithRedisConn executes a function with a Redis connection, handling Redis unavailability gracefully
func (rt *Runtime) WithRedisConn(fn func(redis.Conn) error) error {
	if rt.RP == nil {
		slog.Debug("redis unavailable, skipping operation")
		return nil
	}

	rc := rt.RP.Get()
	defer rc.Close()

	if err := fn(rc); err != nil {
		slog.Warn("redis operation failed", "error", err)
		return nil // don't propagate Redis errors as fatal
	}

	return nil
}
