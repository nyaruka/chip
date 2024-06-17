package testsuite

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

type Client struct {
	conn *websocket.Conn
}

func NewClient(t *testing.T, url string) *Client {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	require.NoError(t, err)

	return &Client{conn: conn}
}

func (c *Client) Send(t *testing.T, d string) {
	err := c.conn.WriteMessage(websocket.TextMessage, []byte(d))
	require.NoError(t, err)

	time.Sleep(100 * time.Millisecond)
}

func (c *Client) Read(t *testing.T) string {
	_, d, err := c.conn.ReadMessage()
	require.NoError(t, err)
	return string(d)
}

func (c *Client) Close(t *testing.T) {
	require.NoError(t, c.conn.Close())
}
