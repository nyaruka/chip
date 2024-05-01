package events

import "time"

type Event interface {
	Type() string
	Time() time.Time
}

type baseEvent struct {
	Type_ string    `json:"type"`
	Time_ time.Time `json:"time"`
}

func (e *baseEvent) Type() string    { return e.Type_ }
func (e *baseEvent) Time() time.Time { return e.Time_ }

type User struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func NewUser(name, email string) *User {
	return &User{Name: name, Email: email}
}
