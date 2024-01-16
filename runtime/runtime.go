package runtime

import (
	"context"
	"database/sql"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/pkg/errors"
)

type Runtime struct {
	DB     *sql.DB
	RP     *redis.Pool
	Config *Config
}

func OpenDBPool(url string, maxOpenConns int) (*sql.DB, error) {
	db, err := sql.Open("postgres", url)
	if err != nil {
		return nil, errors.Wrapf(err, "unable to open database connection: '%s'", url)
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
