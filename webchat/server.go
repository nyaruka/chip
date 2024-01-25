package webchat

import "github.com/nyaruka/tembachat/webchat/events"

type Server interface {
	Start() error
	Stop()

	Connect(Client)
	Disconnect(Client)

	NotifyCourier(Client, events.Event)
}
