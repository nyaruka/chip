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
	contact models.Contact

	inboxQueue     chan events.Event
	inboxStop      chan bool
	inboxWaitGroup sync.WaitGroup
}

func NewClient(s *Server, sock httpx.WebSocket, channel models.Channel, contact models.Contact, isNew bool) *Client {
	c := &Client{
		server:  s,
		socket:  sock,
		channel: channel,
		contact: contact,

		inboxQueue: make(chan events.Event, 10),
		inboxStop:  make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	c.server.Connect(c)

	go c.courierNotifier()

	if isNew {
		// create a chat_started event and send to both client and courier
		evt := events.NewChatStarted(c.contact.ChatID())
		c.Send(evt)
		c.inboxQueue <- evt
	} else {
		evt := events.NewChatResumed(c.contact.ChatID(), c.contact.Email())
		c.Send(evt)
	}

	return c
}

func (c *Client) Channel() models.Channel { return c.channel }
func (c *Client) Contact() models.Contact { return c.contact }

func (c *Client) onMessage(msg []byte) {
	evt, err := events.ReadEvent(msg)
	if err != nil {
		slog.Error("unable to unmarshal event", "chat_id", c.contact.ChatID(), "error", err)
	} else {
		c.inboxQueue <- evt
	}
}

func (c *Client) onClose(code int) {
	c.server.Disconnect(c)

	c.inboxStop <- true
}

func (c *Client) Send(e events.Event) {
	c.socket.Send(jsonx.MustMarshal(e))
}

func (c *Client) Stop() {
	c.socket.Close(1000)

	c.inboxWaitGroup.Wait()
}

func (c *Client) courierNotifier() {
	c.inboxWaitGroup.Add(1)
	defer c.inboxWaitGroup.Done()

	for {
		select {
		case evt := <-c.inboxQueue:
			c.server.onChatReceive(c, evt)
		case <-c.inboxStop:
			return
		}
	}
}
