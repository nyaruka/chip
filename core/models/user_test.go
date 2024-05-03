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

	bobID := testsuite.InsertUser(rt, "bob@nyaruka.com", "Bob", "McFlows", "avatars/1234/1234567890.webp")

	_, err := models.LoadUser(ctx, rt, 1234567)
	assert.EqualError(t, err, "sql: no rows in result set")

	u, err := models.LoadUser(ctx, rt, bobID)
	assert.NoError(t, err)
	assert.Equal(t, bobID, u.ID)
	assert.Equal(t, "bob@nyaruka.com", u.Email)
	assert.Equal(t, "Bob McFlows", u.Name())
	assert.Equal(t, "avatars/1234/1234567890.webp", u.Avatar)
	assert.Equal(t, "http://localhost/media/avatars/1234/1234567890.webp", u.AvatarURL(rt.Config))
}
