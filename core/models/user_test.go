package models_test

import (
	"testing"

	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestLoadUser(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows")

	_, err := models.LoadUser(ctx, rt, 1234567)
	assert.EqualError(t, err, "user query returned no rows")

	u, err := models.LoadUser(ctx, rt, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, u.ID)
	assert.Equal(t, "bob@nyaruka.com", u.Email)
	assert.Equal(t, "Bob McFlows", u.Name())
}
