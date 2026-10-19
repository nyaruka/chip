package cmd

import (
	"testing"

	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/ezconf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	cfg := runtime.NewDefaultConfig()
	require.NoError(t, loadConfig(cfg, []string{"-port=9000", "-courier-url=http://courier.example.com"}))
	assert.Equal(t, 9000, cfg.Port)
	assert.Equal(t, "http://courier.example.com", cfg.CourierURL)

	// an invalid config is an error
	cfg = runtime.NewDefaultConfig()
	err := loadConfig(cfg, []string{"-db=mysql://localhost/temba"})
	assert.ErrorContains(t, err, "invalid config")

	// as is an unknown argument, and usage is asked for with -help
	cfg = runtime.NewDefaultConfig()
	assert.Error(t, loadConfig(cfg, []string{"-nope"}))
	assert.ErrorIs(t, loadConfig(runtime.NewDefaultConfig(), []string{"-help"}), ezconf.ErrHelp)
}
