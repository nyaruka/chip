package web

import (
	"log/slog"
	"sync"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
)

var identifierRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func newIdentifier() string {
	return random.String(24, identifierRunes)
}

type Client interface {
	Channel() models.Channel
	Identifier() string
	Send(e events.Event)
	Stop()
}

type client struct {
	server     Server
	socket     httpx.WebSocket
	channel    models.Channel
	identifier string

	courierQueue     chan events.Event
	courierStop      chan bool
	courierWaitGroup sync.WaitGroup
}

func NewClient(s Server, sock httpx.WebSocket, channel models.Channel, identifier string) Client {
	isNew := false
	if identifier == "" {
		identifier = newIdentifier()
		isNew = true
	}

	c := &client{
		server:     s,
		socket:     sock,
		channel:    channel,
		identifier: identifier,

		courierQueue: make(chan events.Event, 10),
		courierStop:  make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	c.server.Connect(c)

	go c.courierNotifier()

	if isNew {
		// create a chat_started event and send to both client and courier
		evt := events.NewChatStarted(c.Identifier())
		c.Send(evt)
		c.courierQueue <- evt
	} else {
		evt := events.NewChatResumed(c.Identifier())
		c.Send(evt)
	}

	return c
}

func (c *client) Identifier() string { return c.identifier }

func (c *client) Channel() models.Channel { return c.channel }

func (c *client) onMessage(msg []byte) {
	// for now only one type of event supported
	evt := &events.MsgIn{}
	if err := jsonx.Unmarshal(msg, evt); err != nil {
		slog.Error("unable to unmarshal message", "client", c.identifier, "error", err)
	} else {
		c.courierQueue <- evt
	}
}

func (c *client) onClose(code int) {
	c.server.Disconnect(c)

	c.courierStop <- true
}

func (c *client) Send(e events.Event) {
	c.socket.Send(jsonx.MustMarshal(e))
}

func (c *client) Stop() {
	c.socket.Close(1000)

	c.courierWaitGroup.Wait()
}

func (c *client) courierNotifier() {
	c.courierWaitGroup.Add(1)
	defer c.courierWaitGroup.Done()

	for {
		select {
		case evt := <-c.courierQueue:
			c.server.NotifyCourier(c, evt)
		case <-c.courierStop:
			return
		}
	}
}
