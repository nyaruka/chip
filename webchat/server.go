package webchat

type Server interface {
	Start() error
	Stop()

	Register(Client)
	Unregister(Client)

	NotifyCourier(Client, Event)
}
