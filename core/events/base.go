package events

import (
	"encoding/json"

	"github.com/go-playground/validator/v10"
	"github.com/pkg/errors"
)

// TODO move goflow validation utils to gocommon?
var valx = validator.New()

var registeredTypes = map[string](func() Event){}

// registers a new type of event
func registerType(name string, initFunc func() Event) {
	registeredTypes[name] = initFunc
}

type Event interface {
	Type() string
}

type baseEvent struct {
	Type_ string `json:"type" validate:"required"`
}

func (e *baseEvent) Type() string {
	return e.Type_
}

func ReadEvent(d []byte) (Event, error) {
	be := &baseEvent{}
	if err := json.Unmarshal(d, be); err != nil {
		return nil, err
	}

	f := registeredTypes[be.Type_]
	if f == nil {
		return nil, errors.Errorf("unknown event type '%s'", be.Type_)
	}

	e := f()
	if err := json.Unmarshal(d, e); err != nil {
		return nil, err
	}

	return e, valx.Struct(e)
}
