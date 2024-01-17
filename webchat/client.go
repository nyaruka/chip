package webchat

import (
	"log/slog"
	"sync"

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

	courierQueue     chan Event
	courierStop      chan bool
	courierWaitGroup sync.WaitGroup
}

func NewClient(s Server, sock httpx.WebSocket, channel Channel, identifier string) Client {
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

		courierQueue: make(chan Event, 10),
		courierStop:  make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	c.server.Connect(c)

	go c.courierNotifier()

	if isNew {
		// create a chat_started event and send to both client and courier
		evt := NewChatStartedEvent(c.Identifier())
		c.Send(evt)
		c.courierQueue <- evt
	} else {
		evt := NewChatResumedEvent(c.Identifier())
		c.Send(evt)
	}

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
		c.courierQueue <- evt
	}
}

func (c *client) onClose(code int) {
	c.server.Disconnect(c)

	c.courierStop <- true
}

func (c *client) Send(e Event) {
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
