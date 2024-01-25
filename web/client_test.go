package web_test

import (
	"testing"

	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/web"
	"github.com/stretchr/testify/assert"
)

type testChannel struct {
	uuid models.ChannelUUID
}

func (c *testChannel) UUID() models.ChannelUUID { return c.uuid }
func (c *testChannel) OrgID() models.OrgID      { return 0 }
func (c *testChannel) Config() map[string]any   { return nil }

type testServer struct {
	clients map[string]web.Client
}

func (s *testServer) Start() error { return nil }
func (s *testServer) Stop()        {}

func (s *testServer) Connect(c web.Client) {
	s.clients[c.Identifier()] = c
}

func (s *testServer) Disconnect(c web.Client) {
	delete(s.clients, c.Identifier())
}

func (s *testServer) NotifyCourier(web.Client, events.Event) {}

type testSocket struct {
	onMessage func([]byte)
	onClose   func(int)
}

func (s *testSocket) Start()          {}
func (s *testSocket) Send(msg []byte) {}
func (s *testSocket) Close(code int) {
	s.onClose(code)
}

func (s *testSocket) OnMessage(fn func([]byte)) { s.onMessage = fn }
func (s *testSocket) OnClose(fn func(int))      { s.onClose = fn }

func TestClient(t *testing.T) {
	svr := &testServer{clients: map[string]web.Client{}}
	sock := &testSocket{}
	ch := &testChannel{uuid: "d991d239-e4bb-4a93-8c72-e6d093f7b0b8"}

	client := web.NewClient(svr, sock, ch, "65vbbDAQCdPdEWlEhDGy4utO")

	assert.Equal(t, map[string]web.Client{"65vbbDAQCdPdEWlEhDGy4utO": client}, svr.clients)

	client.Stop()

	assert.Equal(t, map[string]web.Client{}, svr.clients)
}
