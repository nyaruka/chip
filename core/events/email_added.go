package events

func init() {
	registerType(TypeEmailAdded, func() Event { return &EmailAdded{} })
}

const TypeEmailAdded string = "email_added"

type EmailAdded struct {
	baseEvent
	Email string `json:"email" validate:"required"`
}
