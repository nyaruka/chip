package webchat

import "github.com/nyaruka/gocommon/uuids"

type Channel interface {
	UUID() uuids.UUID
}

type channel struct {
	uuid uuids.UUID
}

func NewChannel(uuid uuids.UUID) Channel {
	return &channel{uuid: uuid}
}

func (c *channel) UUID() uuids.UUID {
	return c.uuid
}
