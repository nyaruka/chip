package tembachat_test

import (
	"testing"

	"github.com/nyaruka/tembachat"
	"github.com/stretchr/testify/assert"
)

type testServer struct {
	clients map[string]*tembachat.Client
}

func (s *testServer) Start() error { return nil }
func (s *testServer) Stop()        {}

func (s *testServer) Register(c *tembachat.Client) {
	s.clients[c.Identifier()] = c
}

func (s *testServer) Unregister(c *tembachat.Client) {
	delete(s.clients, c.Identifier())
}

func (s *testServer) EventReceived(*tembachat.Client, tembachat.Event) {}

type testSocket struct {
	onMessage func([]byte)
	onClose   func(int)
}

func (s *testSocket) Start()          {}
func (s *testSocket) Send(msg []byte) {}
func (s *testSocket) Close() {
	s.onClose(1000)
}

func (s *testSocket) OnMessage(fn func([]byte)) { s.onMessage = fn }
func (s *testSocket) OnClose(fn func(int))      { s.onClose = fn }

func TestClient(t *testing.T) {
	svr := &testServer{clients: map[string]*tembachat.Client{}}
	sock := &testSocket{}

	client := tembachat.NewClient(svr, sock, "d991d239-e4bb-4a93-8c72-e6d093f7b0b8", "65vbbDAQCdPdEWlEhDGy4utO")

	assert.Equal(t, map[string]*tembachat.Client{"65vbbDAQCdPdEWlEhDGy4utO": client}, svr.clients)

	client.Stop()

	assert.Equal(t, map[string]*tembachat.Client{}, svr.clients)
}
