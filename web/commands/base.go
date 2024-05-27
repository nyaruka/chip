package commands

import (
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
)

// TODO move goflow validation utils to gocommon?
var valx = validator.New()

var registeredTypes = map[string](func() Command){}

// registers a new type of event
func registerType(name string, initFunc func() Command) {
	registeredTypes[name] = initFunc
}

type Command interface {
	Type() string
}

type baseCommand struct {
	Type_ string `json:"type" validate:"required"`
}

func (e *baseCommand) Type() string {
	return e.Type_
}

func ReadCommand(d []byte) (Command, error) {
	be := &baseCommand{}
	if err := json.Unmarshal(d, be); err != nil {
		return nil, err
	}

	f := registeredTypes[be.Type_]
	if f == nil {
		return nil, fmt.Errorf("unknown command type '%s'", be.Type_)
	}

	e := f()
	if err := json.Unmarshal(d, e); err != nil {
		return nil, err
	}

	return e, valx.Struct(e)
}
