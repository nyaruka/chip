package tembachat

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// max time for between reading a message before socket is considered closed
	maxReadWait = 60 * time.Second

	maxWriteWait = 10 * time.Second

	pingPeriod = 30 * time.Second
)

type Socket struct {
	conn             *websocket.Conn
	onMessage        func([]byte)
	onClose          func(int)
	outbox           chan []byte
	readError        chan error
	writeError       chan error
	stopWriter       chan bool
	stopMonitor      chan bool
	rwWaitGroup      sync.WaitGroup
	monitorWaitGroup sync.WaitGroup
}

func NewSocket(c *websocket.Conn, maxReadBytes int64, sendBuffer int) *Socket {
	c.SetReadLimit(maxReadBytes)

	return &Socket{
		conn:        c,
		onMessage:   func([]byte) {},
		onClose:     func(int) {},
		outbox:      make(chan []byte, sendBuffer),
		readError:   make(chan error, 1),
		writeError:  make(chan error, 1),
		stopWriter:  make(chan bool, 1),
		stopMonitor: make(chan bool, 1),
	}
}

func (s *Socket) OnMessage(fn func([]byte)) { s.onMessage = fn }
func (s *Socket) OnClose(fn func(int))      { s.onClose = fn }

func (s *Socket) Start() {
	s.conn.SetReadDeadline(time.Now().Add(maxReadWait))
	s.conn.SetPongHandler(s.pong)

	go s.monitor()
	go s.reader()
	go s.writer()
}

func (s *Socket) Send(msg []byte) {
	s.outbox <- msg
}

func (s *Socket) Close() {
	s.conn.Close() // causes reader to stop
	s.stopWriter <- true
	s.stopMonitor <- true

	s.monitorWaitGroup.Wait()
}

func (s *Socket) pong(m string) error {
	s.conn.SetReadDeadline(time.Now().Add(maxReadWait))

	return nil
}

func (s *Socket) monitor() {
	s.monitorWaitGroup.Add(1)
	defer s.monitorWaitGroup.Done()

	closeCode := websocket.CloseNormalClosure

out:
	for {
		select {
		case err := <-s.readError:
			if e, ok := err.(*websocket.CloseError); ok {
				closeCode = e.Code
			}
			s.stopWriter <- true // ensure writer is stopped
			break out
		case err := <-s.writeError:
			if e, ok := err.(*websocket.CloseError); ok {
				closeCode = e.Code
			}
			s.conn.Close() // ensure reader is stopped
			break out
		case <-s.stopMonitor:
			break out
		}
	}

	s.rwWaitGroup.Wait()

	s.onClose(closeCode)
}

func (s *Socket) reader() {
	s.rwWaitGroup.Add(1)
	defer s.rwWaitGroup.Done()

	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			s.readError <- err
			return
		}

		s.onMessage(message)
	}
}

func (s *Socket) writer() {
	s.rwWaitGroup.Add(1)
	defer s.rwWaitGroup.Done()

	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case msg := <-s.outbox:
			s.conn.SetWriteDeadline(time.Now().Add(maxWriteWait))

			err := s.conn.WriteMessage(websocket.TextMessage, msg)
			if err != nil {
				s.writeError <- err
				return
			}
		case <-ticker.C:
			s.conn.SetWriteDeadline(time.Now().Add(maxWriteWait))

			if err := s.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				s.writeError <- err
				return
			}
		case <-s.stopWriter:
			return
		}
	}
}
