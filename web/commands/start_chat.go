package commands

import "github.com/nyaruka/chip/core/models"

func init() {
	registerType(TypeStartChat, func() Command { return &StartChat{} })
}

const TypeStartChat string = "start_chat"

type StartChat struct {
	baseCommand

	ChatID models.ChatID `json:"chat_id"`
}
