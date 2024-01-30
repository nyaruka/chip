package testsuite

import (
	"context"
	"database/sql"
	"log/slog"
	"os"
	"path"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/redisx/assertredis"
	"github.com/nyaruka/tembachat/runtime"
)

var _db *sql.DB

// Runtime returns the various runtime things a test might need
func Runtime() (context.Context, *runtime.Runtime) {
	cfg := runtime.NewDefaultConfig()

	dbx := getDB()
	rt := &runtime.Runtime{
		DB:     dbx,
		RP:     getRP(),
		Config: cfg,
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})))

	return context.Background(), rt
}

// returns an open test database pool
func getDB() *sql.DB {
	if _db == nil {
		var err error
		_db, err = sql.Open("postgres", "postgres://chatserver_test:temba@localhost/chatserver_test?sslmode=disable&Timezone=UTC")
		noError(err)

		// check if we have tables and if not load test database dump
		_, err = _db.Exec("SELECT * from orgs_org")
		if err != nil {
			ResetDB()
		}
	}
	return _db
}

func ResetDB() {
	// read our schema sql
	sqlSchema, err := os.ReadFile(absPath("testsuite/schema.sql"))
	noError(err)
	_, err = _db.Exec(string(sqlSchema))
	noError(err)
}

// resets our redis database
func ResetRedis() {
	assertredis.FlushDB()
}

// returns a redis pool to our test database
func getRP() *redis.Pool {
	return assertredis.TestDB()
}

// Converts a project root relative path to an absolute path usable in any test. This is needed because go tests
// are run with a working directory set to the current module being tested.
func absPath(p string) string {
	// start in working directory and go up until we are in a directory containing go.mod
	dir, _ := os.Getwd()
	for dir != "/" {
		if _, err := os.Stat(path.Join(dir, "go.mod")); err == nil {
			break
		}
		dir = path.Dir(dir)
	}
	return path.Join(dir, p)
}

// convenience way to call a func and panic if it errors, e.g. must(foo())
func must(err error) {
	if err != nil {
		panic(err)
	}
}

// if just checking an error is nil noError(err) reads better than must(err)
var noError = must
