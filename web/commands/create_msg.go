package commands

func init() {
	registerType(TypeCreateMsg, func() Command { return &CreateMsg{} })
}

const TypeCreateMsg string = "create_msg"

type CreateMsg struct {
	baseCommand

	Text string `json:"text" validate:"required"`
}

func NewCreateMsg(text string) *CreateMsg {
	return &CreateMsg{Text: text}
}
