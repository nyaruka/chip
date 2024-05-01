package events

type Event interface {
	Type() string
}

type baseEvent struct {
	Type_ string `json:"type" validate:"required"`
}

func (e *baseEvent) Type() string {
	return e.Type_
}
