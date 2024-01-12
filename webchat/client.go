package webchat

import (
	"log/slog"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/uuids"
)

var identifierRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func newIdentifier() string {
	return random.String(24, identifierRunes)
}

type Client struct {
	server      Server
	socket      httpx.WebSocket
	channelUUID uuids.UUID
	identifier  string
}

func NewClient(s Server, sock httpx.WebSocket, channelUUID uuids.UUID, identifier string) *Client {
	if identifier == "" {
		identifier = newIdentifier()
	}

	client := &Client{
		server:      s,
		socket:      sock,
		channelUUID: channelUUID,
		identifier:  identifier,
	}

	client.socket.OnMessage(client.onMessage)
	client.socket.OnClose(client.onClose)
	client.socket.Start()

	client.server.Register(client)

	return client
}

func (c *Client) Identifier() string { return c.identifier }

func (c *Client) onMessage(msg []byte) {
	// for now only one type of event supported
	evt := &msgInEvent{}
	if err := jsonx.Unmarshal(msg, evt); err != nil {
		slog.Error("unable to unmarshal message", "client", c.identifier, "error", err)
	} else {
		c.server.EventReceived(c, evt)
	}
}

func (c *Client) onClose(code int) {
	c.server.Unregister(c)
}

func (c *Client) Send(e Event) {
	c.socket.Send(jsonx.MustMarshal(e))
}

func (c *Client) Stop() {
	c.socket.Close(1000)
}
