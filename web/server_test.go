package web_test

import (
	"net/http"
	"testing"

	"github.com/gorilla/websocket"
	"github.com/nyaruka/chip"
	"github.com/nyaruka/chip/testsuite"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	_, rt := testsuite.Runtime()

	mockCourier := testsuite.NewMockCourier(rt)

	svc := chip.NewService(rt, mockCourier)
	assert.NoError(t, svc.Start())

	defer svc.Stop()

	req, _ := http.NewRequest("GET", "http://localhost:8071/", nil)
	trace, err := httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 200, trace.Response.StatusCode)
	assert.Equal(t, `{"version":"Dev"}`, string(trace.ResponseBody))

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "CHP", "WebChat", "123", []string{"webchat"})

	// try to start for a non-existent channel
	req, _ = http.NewRequest("POST", "http://localhost:8071/wc/connect/16955bac-23fd-4b5f-8981-530679ae0ac4/", nil)
	trace, err = httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, `{"error":"no such channel"}`, string(trace.ResponseBody))

	// try to start against an existing channel (still fails because HTTP client does support web sockets)
	req, _ = http.NewRequest("POST", "http://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/", nil)
	trace, err = httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, "Bad Request\n", string(trace.ResponseBody))

	c, _, err := websocket.DefaultDialer.Dial("ws://localhost:8071/wc/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/", nil)
	require.NoError(t, err)
	require.NotNil(t, c)

	c.Close()
}
