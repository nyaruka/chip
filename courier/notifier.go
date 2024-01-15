package courier

import (
	"fmt"
	"sync"

	"github.com/nyaruka/tembachat/webchat"
)

type queuedEvent struct {
	Client webchat.Client
	Event  webchat.Event
}

type Notifier interface {
	Start()
	Notify(webchat.Client, webchat.Event)
	Stop()
}

type notifier struct {
	server  webchat.Server
	baseURL string
	queue   chan queuedEvent
	stop    chan bool
	wg      *sync.WaitGroup
}

func NewNotifier(server webchat.Server, baseURL string, wg *sync.WaitGroup) Notifier {
	return &notifier{
		server:  server,
		baseURL: baseURL,
		queue:   make(chan queuedEvent, 100),
		stop:    make(chan bool),
		wg:      wg,
	}
}

func (n *notifier) Start() {
	n.wg.Add(1)

	go func() {
		defer n.wg.Done()

		for {
			select {
			case qi := <-n.queue:
				n.notify(qi.Client, qi.Event)
			case <-n.stop:
				return
			}
		}
	}()
}

func (n *notifier) Notify(c webchat.Client, e webchat.Event) {
	switch e.(type) {
	case *webchat.ChatStartedEvent, *webchat.MsgInEvent:
		n.queue <- queuedEvent{c, e}
	default:
		panic(fmt.Sprintf("can't send event type %T to courier", e))
	}
}

func (n *notifier) notify(c webchat.Client, e webchat.Event) {
	switch typed := e.(type) {
	case *webchat.ChatStartedEvent:
		callCourier(n.baseURL, c.Channel().UUID(), &courierPayload{
			Type: "chat_started",
			Chat: &courierChat{
				Identifier: c.Identifier(),
			},
		})

	case *webchat.MsgInEvent:
		callCourier(n.baseURL, c.Channel().UUID(), &courierPayload{
			Type: "msg_in",
			Msg: &courierMsg{
				Identifier: c.Identifier(),
				Text:       typed.Text,
			},
		})
	}

}

func (s *notifier) Stop() {
	s.stop <- true
}
