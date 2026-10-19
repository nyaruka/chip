package runtime

import (
	"context"
	"fmt"

	valkey "github.com/gomodule/redigo/redis"
	_ "github.com/lib/pq" // postgres driver
	"github.com/nyaruka/gocommon/aws/s3x"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/vkutil"
	"github.com/vinovest/sqlx"
)

// Runtime represents the set of services required to run chip. Used as a wrapper for those services to simplify
// call signatures.
type Runtime struct {
	Config *Config

	// the platform's database, which chip only ever reads - everything it changes goes through courier
	DB         *sqlx.DB
	VK         *valkey.Pool
	S3         *s3x.Service
	Centrifugo *centrifugo.Service

	HTTP *HTTP
}

func NewRuntime(cfg *Config) (*Runtime, error) {
	rt := &Runtime{Config: cfg}

	var err error

	rt.DB, err = sqlx.Open("postgres", cfg.DB)
	if err != nil {
		return nil, fmt.Errorf("error creating Postgres connection pool: %w", err)
	}
	rt.DB.SetMaxIdleConns(4)
	rt.DB.SetMaxOpenConns(16)

	rt.VK, err = vkutil.NewPool(cfg.Valkey, vkutil.WithMaxActive(16))
	if err != nil {
		return nil, fmt.Errorf("error creating Valkey pool: %w", err)
	}

	// the AWS service constructors resolve credentials and region from the SDK default chain (env
	// vars, instance/task IAM role, shared config/credentials files, etc.)
	rt.S3, err = s3x.NewService(context.Background(), cfg.S3Endpoint, cfg.S3PathStyle)
	if err != nil {
		return nil, fmt.Errorf("error creating S3 service: %w", err)
	}

	rt.Centrifugo = centrifugo.NewService(centrifugo.NewClient(cfg.CentrifugoEndpoint, cfg.CentrifugoKey), rt.VK)

	rt.HTTP = newHTTP()

	return rt, nil
}

func (r *Runtime) Stop() {
	r.DB.Close()
	r.VK.Close()
}
