package tembachat

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/gocommon/random"
	"github.com/nyaruka/gocommon/uuids"
)

type Client struct {
	server      *Server
	conn        *websocket.Conn
	channelUUID uuids.UUID
	identifier  string
	outbox      chan Event
}

func NewClient(s *Server, c *websocket.Conn, channelUUID uuids.UUID, identifier string) *Client {
	if identifier == "" {
		identifier = newIdentifier()
	}

	return &Client{
		server:      s,
		conn:        c,
		channelUUID: channelUUID,
		identifier:  identifier,
		outbox:      make(chan Event, 10),
	}
}

func (c *Client) Start() {
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(c.pong)

	c.server.wg.Add(2)

	c.Send(newChatStartedEvent(c.identifier))

	go c.readUntilClose()
	go c.writeUntilClose()
}

func (c *Client) readUntilClose() {
	log := slog.With("client", c.identifier)

	defer func() {
		close(c.outbox) // tells message writing loop to stop

		c.server.unregister(c)
		c.server.wg.Done()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error("unexpected connection closure", "error", err)
			}
			break
		}

		// for now only one type of event supported
		evt := &msgInEvent{}
		if err := jsonx.Unmarshal(message, evt); err != nil {
			log.Error("unable to unmarshal message", "error", err)
		} else {
			c.server.eventReceived(c, evt)
		}
	}
}

func (c *Client) writeUntilClose() {
	log := slog.With("client", c.identifier)
	ticker := time.NewTicker(30 * time.Second)

	defer func() {
		ticker.Stop()

		c.server.wg.Done()
	}()

	for {
		select {
		case event, ok := <-c.outbox:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			// outbox channel has been closed
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, jsonx.MustMarshal(event))
			if err != nil {
				log.Error("error writing message", "error", err)
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) pong(m string) error {
	log := slog.With("client", c.identifier)

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	log.Debug("pong received")
	return nil
}

func (c *Client) Send(e Event) {
	c.outbox <- e
}

func (c *Client) Stop() {
	c.conn.Close()
}

var identifierRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

func newIdentifier() string {
	return random.String(24, identifierRunes)
}
