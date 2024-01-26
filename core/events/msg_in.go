package events

func init() {
	registerType(TypeMsgIn, func() Event { return &MsgIn{} })
}

const TypeMsgIn string = "msg_in"

type MsgIn struct {
	baseEvent
	Text string `json:"text" validate:"required"`
}
