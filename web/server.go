package web

import "github.com/nyaruka/tembachat/core/events"

type Server interface {
	Start() error
	Stop()

	Connect(Client)
	Disconnect(Client)

	NotifyCourier(Client, events.Event)
}
