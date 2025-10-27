package commands

import "github.com/nyaruka/chip/core/models"

func init() {
	registerType(TypeAckChat, func() Command { return &AckChat{} })
}

const TypeAckChat string = "ack_chat"

type AckChat struct {
	baseCommand

	MsgUUID models.MsgUUID `json:"msg_uuid" validate:"required"`
}
