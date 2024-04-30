package commands

func init() {
	registerType(TypeSetEmail, func() Command { return &SetEmail{} })
}

const TypeSetEmail string = "set_email"

type SetEmail struct {
	baseCommand

	Email string `json:"email" validate:"required"`
}
