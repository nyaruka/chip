package chip_test

import (
	"testing"

	"github.com/nyaruka/chip"
	"github.com/nyaruka/chip/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestService(t *testing.T) {
	svc := chip.NewService(testsuite.Config())
	assert.NoError(t, svc.Start())

	svc.Stop()
}
