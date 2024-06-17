package commands

import "github.com/nyaruka/chip/core/models"

func init() {
	registerType(TypeAckMsg, func() Command { return &AckMsg{} })
}

const TypeAckMsg string = "ack_msg"

type AckMsg struct {
	baseCommand

	MsgID models.MsgID `json:"msg_id" validate:"required"`
}
