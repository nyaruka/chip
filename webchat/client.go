package webchat

import (
	"log/slog"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/random"
)

var identifierRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func newIdentifier() string {
	return random.String(24, identifierRunes)
}

type Client interface {
	Channel() Channel
	Identifier() string
	Send(e Event)
	Stop()
}

type client struct {
	server     Server
	socket     httpx.WebSocket
	channel    Channel
	identifier string
}

func NewClient(s Server, sock httpx.WebSocket, channel Channel, identifier string) Client {
	if identifier == "" {
		identifier = newIdentifier()
	}

	c := &client{
		server:     s,
		socket:     sock,
		channel:    channel,
		identifier: identifier,
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	c.server.Register(c)

	return c
}

func (c *client) Identifier() string { return c.identifier }

func (c *client) Channel() Channel { return c.channel }

func (c *client) onMessage(msg []byte) {
	// for now only one type of event supported
	evt := &MsgInEvent{}
	if err := jsonx.Unmarshal(msg, evt); err != nil {
		slog.Error("unable to unmarshal message", "client", c.identifier, "error", err)
	} else {
		c.server.EventReceived(c, evt)
	}
}

func (c *client) onClose(code int) {
	c.server.Unregister(c)
}

func (c *client) Send(e Event) {
	c.socket.Send(jsonx.MustMarshal(e))
}

func (c *client) Stop() {
	c.socket.Close(1000)
}
