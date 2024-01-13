package runtime

import "github.com/gomodule/redigo/redis"

type Runtime struct {
	RP *redis.Pool
}
