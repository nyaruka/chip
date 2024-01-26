package web

import (
	"log/slog"
	"sync"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
)

var identifierRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func newIdentifier() string {
	return random.String(24, identifierRunes)
}

type Client struct {
	server  *Server
	socket  httpx.WebSocket
	channel models.Channel

	chatID string
	email  string

	inboxQueue     chan events.Event
	inboxStop      chan bool
	inboxWaitGroup sync.WaitGroup
}

func NewClient(s *Server, sock httpx.WebSocket, channel models.Channel, chatID, email string) *Client {
	isNew := false
	if chatID == "" {
		chatID = newIdentifier()
		isNew = true
	}

	c := &Client{
		server:  s,
		socket:  sock,
		channel: channel,
		chatID:  chatID,
		email:   email,

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
		evt := events.NewChatStarted(chatID)
		c.Send(evt)
		c.inboxQueue <- evt
	} else {
		evt := events.NewChatResumed(chatID, email)
		c.Send(evt)
	}

	return c
}

func (c *Client) Channel() models.Channel { return c.channel }
func (c *Client) ChatID() string          { return c.chatID }
func (c *Client) Email() string           { return c.email }
func (c *Client) URN() urns.URN           { return models.NewURN(c.chatID, c.email) }

func (c *Client) onMessage(msg []byte) {
	evt, err := events.ReadEvent(msg)
	if err != nil {
		slog.Error("unable to unmarshal event", "chat_id", c.chatID, "error", err)
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
