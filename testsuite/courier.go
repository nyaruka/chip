package testsuite

import (
	"fmt"

	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/urns"
	"github.com/nyaruka/tembachat/core/models"
	"github.com/nyaruka/tembachat/runtime"
)

type MockCourier struct {
	rt    *runtime.Runtime
	Calls []string
}

func NewMockCourier(rt *runtime.Runtime) *MockCourier {
	return &MockCourier{rt: rt}
}

func (c *MockCourier) StartChat(ch *models.Channel, chatID models.ChatID) error {
	c.Calls = append(c.Calls, fmt.Sprintf("StartChat(%s, %s)", ch.UUID, chatID))

	cid := InsertContact(c.rt, ch.OrgID, "")
	InsertURN(c.rt, ch.OrgID, cid, urns.URN(fmt.Sprintf("webchat:%s", chatID)))

	return nil
}

func (c *MockCourier) CreateMsg(ch *models.Channel, contact *models.Contact, text string, attachments []string) error {
	c.Calls = append(c.Calls, fmt.Sprintf("CreateMsg(%s, %d, '%s')", ch.UUID, contact.ID, text))

	InsertIncomingMsg(c.rt, ch.OrgID, ch.ID, contact.ID, contact.URNID, text, dates.Now())

	return nil
}
