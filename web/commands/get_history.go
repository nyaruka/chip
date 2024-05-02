package commands

import "time"

func init() {
	registerType(TypeGetHistory, func() Command { return &GetHistory{} })
}

const TypeGetHistory string = "get_history"

type GetHistory struct {
	baseCommand

	Before time.Time `json:"before" validate:"required"`
}
