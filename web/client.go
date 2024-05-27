package web

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/web/commands"
	"github.com/nyaruka/chip/web/events"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/uuids"
)

type Client struct {
	id      string
	server  *Server
	socket  httpx.WebSocket
	channel *models.Channel
	contact *models.Contact

	send     chan events.Event
	sendStop chan bool
	sendWait sync.WaitGroup
}

func NewClient(s *Server, sock httpx.WebSocket, channel *models.Channel) *Client {
	c := &Client{
		id:      string(uuids.New()),
		server:  s,
		socket:  sock,
		channel: channel,

		send:     make(chan events.Event, 16),
		sendStop: make(chan bool),
	}

	c.socket.OnMessage(c.onMessage)
	c.socket.OnClose(c.onClose)
	c.socket.Start()

	go c.sender()

	return c
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
	log := c.log().With("command", cmd.Type())

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	switch typed := cmd.(type) {
	case *commands.StartChat:
		if c.contact != nil {
			log.Debug("chat already started, command ignored")
			return nil
		}

		// if client provided a chat ID look for a matching contact
		if typed.ChatID != "" {
			contact, err := models.LoadContact(ctx, c.server.rt, c.channel.OrgID, typed.ChatID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("error looking up contact: %w", err)
			}

			if contact != nil {
				c.contact = contact
				c.Send(events.NewChatResumed(dates.Now(), contact.ChatID, contact.Email))
				return nil
			}
		}

		// if not generate a new random chat id
		chatID := models.NewChatID()

		// and have courier create a contact and trigger a new_conversation event
		if err := c.server.service.Courier().StartChat(c.channel, chatID); err != nil {
			return fmt.Errorf("error notifying courier: %w", err)
		}

		// contact should now exist now...
		contact, err := models.LoadContact(ctx, c.server.rt, c.channel.OrgID, chatID)
		if err != nil {
			return fmt.Errorf("error looking up new contact: %w", err)
		}

		c.contact = contact
		c.Send(events.NewChatStarted(dates.Now(), contact.ChatID))

	case *commands.SendMsg:
		if c.contact == nil {
			log.Debug("chat not started, command ignored")
			return nil
		}
		if typed.Text == "" && len(typed.Attachments) == 0 {
			log.Debug("msg is empty, command ignored")
			return nil
		}

		if err := c.server.service.Courier().CreateMsg(c.channel, c.contact, typed.Text, typed.Attachments); err != nil {
			return fmt.Errorf("error notifying courier, %w", err)
		}

	case *commands.GetHistory:
		if c.contact == nil {
			log.Debug("chat not started, command ignored")
			return nil
		}

		msgs, err := models.LoadContactMessages(ctx, c.server.rt, c.contact.ID, typed.Before, 25)
		if err != nil {
			return fmt.Errorf("error loading contact messages: %w", err)

		}

		history := make([]events.Event, len(msgs))
		for i, m := range msgs {
			if m.Direction == models.DirectionOut {
				// TODO find logical place for this so that it can be shared with Service.send
				var user *events.User
				if m.CreatedByID != models.NilUserID {
					u, err := c.server.service.Store().GetUser(ctx, m.CreatedByID)
					if err != nil {
						log.Error("error fetching user", "error", err)
					} else {
						user = events.NewUser(u.Name(), u.Email, u.AvatarURL(c.server.rt.Config))
					}
				}
				history[i] = events.NewMsgOut(m.CreatedOn, m.ID, m.Text, m.Attachments, m.Origin(), user)
			} else {
				history[i] = events.NewMsgIn(m.CreatedOn, m.ID, m.Text)
			}
		}

		c.Send(events.NewHistory(dates.Now(), history))

	case *commands.SetEmail:
		if c.contact == nil {
			log.Debug("chat not started, command ignored")
			return nil
		}

		if err := c.contact.UpdateEmail(ctx, c.server.rt, typed.Email); err != nil {
			return fmt.Errorf("error updating email: %w", err)
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
	return slog.With("client_id", c.id, "channel", c.channel.UUID, "chat_id", c.chatID())
}
