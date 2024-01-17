package webchat_test

import (
	"testing"

	"github.com/nyaruka/tembachat/webchat"
	"github.com/stretchr/testify/assert"
)

type testChannel struct {
	uuid webchat.ChannelUUID
}

func (c *testChannel) UUID() webchat.ChannelUUID { return c.uuid }
func (c *testChannel) OrgID() webchat.OrgID      { return 0 }
func (c *testChannel) Config() map[string]any    { return nil }

type testServer struct {
	clients map[string]webchat.Client
}

func (s *testServer) Start() error { return nil }
func (s *testServer) Stop()        {}

func (s *testServer) Connect(c webchat.Client) {
	s.clients[c.Identifier()] = c
}

func (s *testServer) Disconnect(c webchat.Client) {
	delete(s.clients, c.Identifier())
}

func (s *testServer) NotifyCourier(webchat.Client, webchat.Event) {}

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
	svr := &testServer{clients: map[string]webchat.Client{}}
	sock := &testSocket{}
	ch := &testChannel{uuid: "d991d239-e4bb-4a93-8c72-e6d093f7b0b8"}

	client := webchat.NewClient(svr, sock, ch, "65vbbDAQCdPdEWlEhDGy4utO")

	assert.Equal(t, map[string]webchat.Client{"65vbbDAQCdPdEWlEhDGy4utO": client}, svr.clients)

	client.Stop()

	assert.Equal(t, map[string]webchat.Client{}, svr.clients)
}
