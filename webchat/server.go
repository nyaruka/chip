package webchat

type Server interface {
	Start() error
	Stop()

	Connect(Client)
	Disconnect(Client)

	NotifyCourier(Client, Event)
}
