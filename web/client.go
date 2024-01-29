package web

import (
	"log/slog"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
)

type Client struct {
	server  *Server
	socket  httpx.WebSocket
	channel models.Channel
	contact *models.Contact
}

func NewClient(s *Server, sock httpx.WebSocket, channel models.Channel, contact *models.Contact, isNew bool) *Client {
	c := &Client{
		server:  s,
		socket:  sock,
		channel: channel,
		contact: contact,
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	c.server.Connect(c)

	if isNew {
		c.Send(events.NewChatStarted(contact.ChatID))
	} else {
		c.Send(events.NewChatResumed(contact.ChatID, contact.Email))
	}

	return c
}

func (c *Client) onMessage(msg []byte) {
	evt, err := events.ReadEvent(msg)
	if err != nil {
		slog.Error("unable to unmarshal event", "chat_id", c.contact.ChatID, "error", err)
	} else {
		c.server.service.OnChatReceive(c.channel, c.contact, evt)
	}
}

func (c *Client) onClose(code int) {
	c.server.Disconnect(c)
}

func (c *Client) Send(e events.Event) {
	c.socket.Send(jsonx.MustMarshal(e))
}

func (c *Client) Stop() {
	c.socket.Close(1000)
}
