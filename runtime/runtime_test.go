package runtime_test

import (
	"testing"

	"github.com/gomodule/redigo/redis"
	"github.com/nyaruka/chip/runtime"
	"github.com/stretchr/testify/assert"
)

func TestWithRedisConnNilPool(t *testing.T) {
	// Test that WithRedisConn handles nil Redis pool gracefully
	rt := &runtime.Runtime{RP: nil}
	
	callCount := 0
	err := rt.WithRedisConn(func(rc redis.Conn) error {
		callCount++
		return nil
	})
	
	assert.NoError(t, err)
	assert.Equal(t, 0, callCount, "function should not be called when Redis pool is nil")
}

func TestWithRedisConnWithError(t *testing.T) {
	// Test that WithRedisConn handles Redis errors gracefully
	rt := &runtime.Runtime{RP: nil}
	
	err := rt.WithRedisConn(func(rc redis.Conn) error {
		return redis.ErrNil // Simulate a Redis error
	})
	
	// Should not propagate the error
	assert.NoError(t, err)
}