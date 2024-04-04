package models_test

import (
	"testing"
	"time"

	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
)

func TestLoadContactMessages(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	chanID := testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})
	annID := testsuite.InsertContact(rt, orgID, "Ann")
	annURNID := testsuite.InsertURN(rt, orgID, annID, "webchat:78cddDAQCdPdEWlEhDGy4utO")
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	bobURNID := testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	msgs, err := models.LoadContactMessages(ctx, rt, bobID, nil, models.NilMsgID)
	assert.NoError(t, err)
	assert.Len(t, msgs, 0)

	t1 := time.Date(2024, 4, 5, 17, 12, 45, 123456789, time.UTC)
	t2 := time.Date(2024, 4, 5, 17, 13, 45, 123456789, time.UTC)
	t3 := time.Date(2024, 4, 5, 17, 14, 45, 123456789, time.UTC)

	msg1ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "Hello", t1)
	msg2ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "There", t2)
	msg3ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "World", t2)
	msg4ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "!!!", t3)
	testsuite.InsertIncomingMsg(rt, orgID, chanID, annID, annURNID, "Hello", time.Date(2024, 4, 5, 17, 12, 45, 123456789, time.UTC))

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, nil, models.NilMsgID)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 4) {
		assert.Equal(t, msg4ID, msgs[0].ID)
		assert.Equal(t, "!!!", msgs[0].Text)
		assert.Equal(t, msg3ID, msgs[1].ID)
		assert.Equal(t, "World", msgs[1].Text)
		assert.Equal(t, msg2ID, msgs[2].ID)
		assert.Equal(t, "There", msgs[2].Text)
		assert.Equal(t, msg1ID, msgs[3].ID)
		assert.Equal(t, "Hello", msgs[3].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, &t3, msg4ID)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 3) {
		assert.Equal(t, "World", msgs[0].Text)
		assert.Equal(t, "There", msgs[1].Text)
		assert.Equal(t, "Hello", msgs[2].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, &t2, msg3ID)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 2) {
		assert.Equal(t, "There", msgs[0].Text)
		assert.Equal(t, "Hello", msgs[1].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, &t2, msg2ID)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 1) {
		assert.Equal(t, "Hello", msgs[0].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, &t1, msg1ID)
	assert.NoError(t, err)
	assert.Len(t, msgs, 0)
}
