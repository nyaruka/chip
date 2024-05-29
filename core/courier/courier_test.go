package courier_test

import (
	"testing"

	"github.com/nyaruka/chip/core/courier"
	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCourier(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	defer httpx.SetRequestor(httpx.DefaultRequestor)
	mocks := httpx.NewMockRequestor(map[string][]*httpx.MockResponse{
		"http://example.com/c/chp/8291264a-4581-4d12-96e5-e9fcfa6e68d9/receive": {
			httpx.NewMockResponse(200, nil, nil),
			httpx.NewMockResponse(200, nil, nil),
			httpx.NewMockResponse(400, nil, nil),
		},
	})
	httpx.SetRequestor(mocks)

	c := courier.NewCourier(&runtime.Config{Domain: "example.com"})

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "Web Chat", "", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	channel, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	require.NoError(t, err)

	bob, err := models.LoadContact(ctx, rt, orgID, "65vbbDAQCdPdEWlEhDGy4utO")
	require.NoError(t, err)

	err = c.StartChat(channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, "POST", mocks.Requests()[0].Method)

	err = c.CreateMsg(channel, bob, "hello")
	assert.NoError(t, err)
	assert.Equal(t, "POST", mocks.Requests()[1].Method)

	err = c.StartChat(channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.EqualError(t, err, "courier returned non-2XX status")

	assert.False(t, mocks.HasUnused())
}
