package tembachat

import (
	"log/slog"
	"time"

	"github.com/gorilla/websocket"
	"github.com/nyaruka/gocommon/uuids"
)

type Client struct {
	server     *Server
	conn       *websocket.Conn
	identifier string
	outbox     chan string
}

func NewClient(s *Server, c *websocket.Conn) *Client {
	return &Client{
		server:     s,
		conn:       c,
		identifier: string(uuids.New()),
		outbox:     make(chan string, 10),
	}
}

func (c *Client) Start() {
	c.conn.SetReadLimit(4096)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(c.pong)

	c.server.wg.Add(2)

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

		c.server.messageReceived(c, string(message))
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
		case message, ok := <-c.outbox:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))

			// outbox channel has been closed
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			err := c.conn.WriteMessage(websocket.TextMessage, []byte(message))
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

func (c *Client) Send(msg string) {
	c.outbox <- msg
}

func (c *Client) Stop() {
	c.conn.Close()
}
