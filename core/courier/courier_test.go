package courier_test

import (
	"testing"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/tembachat/core/courier"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCourier(t *testing.T) {
	ctx, rt := testsuite.Runtime()

	defer testsuite.ResetDB()

	defer httpx.SetRequestor(httpx.DefaultRequestor)
	mocks := httpx.NewMockRequestor(map[string][]*httpx.MockResponse{
		"http://courier.com/c/twc/8291264a-4581-4d12-96e5-e9fcfa6e68d9/receive": {
			httpx.NewMockResponse(200, nil, nil),
			httpx.NewMockResponse(200, nil, nil),
			httpx.NewMockResponse(400, nil, nil),
		},
	})
	httpx.SetRequestor(mocks)

	c := courier.NewCourier(&runtime.Config{Courier: "http://courier.com"})

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})
	bobID := testsuite.InsertContact(rt, orgID, "Bob")
	testsuite.InsertURN(rt, orgID, bobID, "webchat:65vbbDAQCdPdEWlEhDGy4utO")

	channel, err := models.LoadChannel(ctx, rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9")
	require.NoError(t, err)

	bob, err := models.LoadContact(ctx, rt, orgID, "65vbbDAQCdPdEWlEhDGy4utO")
	require.NoError(t, err)

	err = c.StartChat(channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.NoError(t, err)
	assert.Equal(t, "POST", mocks.Requests()[0].Method)

	err = c.CreateMsg(channel, bob, "hello", nil)
	assert.NoError(t, err)
	assert.Equal(t, "POST", mocks.Requests()[1].Method)

	err = c.StartChat(channel, "65vbbDAQCdPdEWlEhDGy4utO")
	assert.EqualError(t, err, "courier returned non-2XX status")

	assert.False(t, mocks.HasUnused())
}
