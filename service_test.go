package tembachat_test

import (
	"testing"

	"github.com/nyaruka/tembachat"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestService(t *testing.T) {
	svc := tembachat.NewService(testsuite.Config())
	assert.NoError(t, svc.Start())

	svc.Stop()
}
