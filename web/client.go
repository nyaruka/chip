package web

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/web/commands"
	"github.com/nyaruka/chip/web/events"
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

		contact, isNew, err := c.server.service.StartChat(ctx, c.channel, typed.ChatID)
		if err != nil {
			return fmt.Errorf("error from service: %w", err)
		}

		c.contact = contact

		if isNew {
			c.Send(events.NewChatStarted(contact.ChatID))
		} else {
			c.Send(events.NewChatResumed(contact.ChatID, contact.Email))
		}

	case *commands.SendMsg:
		if c.contact == nil {
			log.Debug("chat not started, command ignored")
			return nil
		}

		if err := c.server.service.CreateMsgIn(ctx, c.channel, c.contact, typed.Text); err != nil {
			return fmt.Errorf("error from service: %w", err)
		}

	case *commands.AckChat:
		if c.contact == nil {
			log.Debug("chat not started, command ignored")
			return nil
		}

		if err := c.server.service.ConfirmMsgOut(ctx, c.channel, c.contact, typed.MsgID); err != nil {
			return fmt.Errorf("error from service: %w", err)
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

		history := make([]*events.HistoryItem, len(msgs))
		for i, m := range msgs {
			if m.Direction == models.DirectionOut {
				msgOut, err := m.ToMsgOut(ctx, c.server.service.Store())
				if err != nil {
					return fmt.Errorf("error converting outbound message: %w", err)
				}

				history[i] = &events.HistoryItem{MsgOut: msgOut}
			} else {
				history[i] = &events.HistoryItem{MsgIn: m.ToMsgIn()}
			}
		}

		c.Send(events.NewHistory(history))

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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if c.contact != nil {
		c.server.service.CloseChat(ctx, c.channel, c.contact)
	}

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
