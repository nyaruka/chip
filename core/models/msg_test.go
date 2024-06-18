package models_test

import (
	"testing"
	"time"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadContactMessages(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	chanID := testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})
	annID := testsuite.InsertContact(rt, orgID, "Ann")
	annURNID := testsuite.InsertURN(rt, orgID, annID, "webchat:78cddDAQCdPdEWlEhDGy4utO")
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	bobURNID := testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	msgs, err := models.LoadContactMessages(ctx, rt, bobID, time.Now(), 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 0)

	t1 := time.Date(2024, 4, 5, 17, 12, 45, 123456789, time.UTC)
	t2 := time.Date(2024, 4, 5, 17, 13, 45, 123456789, time.UTC)
	t3 := time.Date(2024, 4, 5, 17, 14, 45, 123456789, time.UTC)

	msg1ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "Hello", t1)
	msg2ID := testsuite.InsertOutgoingMsg(rt, orgID, chanID, bobID, bobURNID, "There", t2)
	msg3ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "World", t3)
	testsuite.InsertIncomingMsg(rt, orgID, chanID, annID, annURNID, "Hello", time.Date(2024, 4, 5, 17, 12, 45, 123456789, time.UTC))

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, time.Now(), 10)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 3) {
		assert.Equal(t, msg3ID, msgs[0].ID)
		assert.Equal(t, "World", msgs[0].Text)
		assert.Equal(t, models.DirectionIn, msgs[0].Direction)

		assert.Equal(t, msg2ID, msgs[1].ID)
		assert.Equal(t, "There", msgs[1].Text)
		assert.Equal(t, models.DirectionOut, msgs[1].Direction)

		assert.Equal(t, msg1ID, msgs[2].ID)
		assert.Equal(t, "Hello", msgs[2].Text)
		assert.Equal(t, models.DirectionIn, msgs[2].Direction)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, t3, 10)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 2) {
		assert.Equal(t, "There", msgs[0].Text)
		assert.Equal(t, "Hello", msgs[1].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, t3, 1)
	assert.NoError(t, err)
	if assert.Len(t, msgs, 1) {
		assert.Equal(t, "There", msgs[0].Text)
	}

	msgs, err = models.LoadContactMessages(ctx, rt, bobID, t1, 10)
	assert.NoError(t, err)
	assert.Len(t, msgs, 0)
}

func TestDMMsgToMsgInAndOut(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	chanID := testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	bobURNID := testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	msg1ID := testsuite.InsertIncomingMsg(rt, orgID, chanID, bobID, bobURNID, "Hello", time.Now())
	msg2ID := testsuite.InsertOutgoingMsg(rt, orgID, chanID, bobID, bobURNID, "There", time.Now())
	msgs, err := models.LoadContactMessages(ctx, rt, bobID, time.Now(), 10)
	require.NoError(t, err)
	msg1 := msgs[1]
	msg2 := msgs[0]

	store := models.NewStore(rt)
	store.Start()
	defer store.Stop()

	msg1In := msg1.ToMsgIn()
	assert.Equal(t, models.NewMsgIn(msg1ID, "Hello", msg1.CreatedOn), msg1In)

	msg2Out, err := msg2.ToMsgOut(ctx, store)
	assert.NoError(t, err)
	assert.Equal(t, models.NewMsgOut(msg2ID, "There", nil, "chat", nil, msg2.CreatedOn), msg2Out)

	// can't call ToMsgIn on an outbound message and vice versa
	assert.Panics(t, func() { msg2.ToMsgIn() })
	assert.Panics(t, func() { msg1.ToMsgOut(ctx, store) })
}
