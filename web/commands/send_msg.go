package commands

func init() {
	registerType(TypeSendMsg, func() Command { return &SendMsg{} })
}

const TypeSendMsg string = "send_msg"

type SendMsg struct {
	baseCommand

	Text string `json:"text" validate:"required"`
}
