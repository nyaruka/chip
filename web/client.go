package web

import (
	"context"
	"database/sql"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/tembachat/core/courier"
	"github.com/nyaruka/tembachat/core/events"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/web/commands"
	"github.com/pkg/errors"
)

type Client struct {
	clientID string
	server   *Server
	socket   httpx.WebSocket
	channel  models.Channel
	contact  *models.Contact

	send     chan events.Event
	sendStop chan bool
	sendWait sync.WaitGroup
}

func NewClient(s *Server, sock httpx.WebSocket, channel models.Channel) *Client {
	c := &Client{
		clientID: string(uuids.New()),
		server:   s,
		socket:   sock,
		channel:  channel,

		send:     make(chan events.Event, 16),
		sendStop: make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	go c.sender()

	return c
}

func (c *Client) Channel() models.Channel {
	return c.channel
}

func (c *Client) onMessage(msg []byte) {
	log := c.log()

	cmd, err := commands.ReadCommand(msg)
	if err != nil {
		log.Error("unable to unmarshal command", "error", err)
		return
	}

	if err = c.onCommand(cmd); err != nil {
		log.Error("error handling command", "command", cmd.Type(), "error", err)
	}
}

func (c *Client) onCommand(cmd commands.Command) error {
	log := c.log()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch typed := cmd.(type) {
	case *commands.StartChat:
		if c.contact != nil {
			log.Debug("chat already started")
			return nil
		}

		// if client provided a chat ID look for a matching contact
		if typed.ChatID != "" {
			contact, err := models.LoadContact(ctx, c.server.rt, c.channel, typed.ChatID)
			if err != nil && err != sql.ErrNoRows {
				return errors.Wrap(err, "error looking up contact")
			}

			if contact != nil {
				c.contact = contact
				c.Send(events.NewChatResumed(contact.ChatID, contact.Email))
				return nil
			}
		}

		// if not generate a new random chat id
		chatID := models.NewChatID()

		// and have courier create a contact and trigger a new_conversation event
		if err := courier.StartChat(c.server.rt.Config, c.channel, chatID); err != nil {
			return errors.Wrap(err, "error notifying courier")
		}

		// contact should now exist now...
		contact, err := models.LoadContact(ctx, c.server.rt, c.channel, chatID)
		if err != nil {
			return errors.Wrap(err, "error looking up new contact")
		}

		c.contact = contact
		c.Send(events.NewChatStarted(contact.ChatID))

	case *commands.CreateMsg:
		if c.contact == nil {
			log.Debug("chat not started, msg event ignored")
			return nil
		}

		if err := courier.CreateMsg(c.server.rt.Config, c.channel, c.contact, typed.Text); err != nil {
			return errors.Wrap(err, "error notifying courier")
		}

	case *commands.SetEmail:
		if c.contact == nil {
			log.Debug("chat not started, set email event ignored")
			return nil
		}

		if err := c.contact.UpdateEmail(ctx, c.server.rt, typed.Email); err != nil {
			return errors.Wrap(err, "error updating email")
		}
	}

	return nil
}

func (c *Client) onClose(code int) {
	c.log().Info("closing", "code", code)

	c.server.OnDisconnect(c)

	c.sendStop <- true
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

func (c *Client) chatID() models.ChatID {
	if c.contact != nil {
		return c.contact.ChatID
	}
	return ""
}

func (c *Client) log() *slog.Logger {
	return slog.With("client_id", c.clientID, "channel", c.channel.UUID(), "chat_id", c.chatID())
}
