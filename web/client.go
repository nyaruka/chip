package web

import (
	"log/slog"
	"sync"

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

	send     chan events.Event
	sendStop chan bool
	sendWait sync.WaitGroup
}

func NewClient(s *Server, sock httpx.WebSocket, channel models.Channel, contact *models.Contact, isNew bool) *Client {
	c := &Client{
		server:  s,
		socket:  sock,
		channel: channel,
		contact: contact,

		send:     make(chan events.Event, 1), // allow buffering of one outgoing event at most
		sendStop: make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	go c.sender()

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
	c.server.OnDisconnect(c)

	c.sendStop <- true
}

// CanSend returns whether Send can be called without blocking.
func (c *Client) CanSend() bool {
	return len(c.send) == 0
}

func (c *Client) Send(e events.Event) {
	c.send <- e
}

func (c *Client) Stop() {
	c.socket.Close(1000)

	c.sendWait.Wait()
}

func (c *Client) sender() {
	c.sendWait.Add(1)
	defer c.sendWait.Done()

	for {
		select {
		case e := <-c.send:
			c.socket.Send(jsonx.MustMarshal(e))
		case <-c.sendStop:
			return
		}
	}
}
