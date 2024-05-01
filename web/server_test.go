package web_test

import (
	"net/http"
	"testing"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/testsuite"
	"github.com/nyaruka/tembachat/web"
	"github.com/stretchr/testify/assert"
)

type MockService struct {
	store models.Store
}

func (s *MockService) Store() models.Store                                         { return s.store }
func (s *MockService) OnChatStarted(models.Channel, *models.Contact)               {}
func (s *MockService) OnChatReceive(models.Channel, *models.Contact, events.Event) {}
func (s *MockService) OnSendRequest(models.Channel, *models.MsgOut)                {}

func TestServer(t *testing.T) {
	_, rt := testsuite.Runtime()

	mockSvc := &MockService{store: models.NewStore(rt)}

	server := web.NewServer(rt, mockSvc)
	server.Start()
	defer server.Stop()

	orgID := testsuite.InsertOrg(rt, "Nyaruka")
	testsuite.InsertChannel(rt, "8291264a-4581-4d12-96e5-e9fcfa6e68d9", orgID, "TWC", "WebChat", "123", []string{"webchat"})

	// try to start for a non-existent channel
	req, _ := http.NewRequest("POST", "http://localhost:8070/connect/16955bac-23fd-4b5f-8981-530679ae0ac4/", nil)
	trace, err := httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, `{"error":"no such channel"}`, string(trace.ResponseBody))

	// try to start against an existing channel (still fails because client does support web sockets)
	req, _ = http.NewRequest("POST", "http://localhost:8070/connect/8291264a-4581-4d12-96e5-e9fcfa6e68d9/", nil)
	trace, err = httpx.DoTrace(http.DefaultClient, req, nil, nil, -1)
	assert.NoError(t, err)
	assert.Equal(t, 400, trace.Response.StatusCode)
	assert.Equal(t, "Bad Request\n", string(trace.ResponseBody))
}
