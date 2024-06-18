package testsuite

import (
	"context"
	"fmt"

	"github.com/nyaruka/chip/core/models"
	"github.com/nyaruka/chip/runtime"
	"github.com/nyaruka/gocommon/dates"
	"github.com/nyaruka/gocommon/urns"
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

func (c *MockCourier) CreateMsg(ch *models.Channel, contact *models.Contact, text string) error {
	c.Calls = append(c.Calls, fmt.Sprintf("CreateMsg(%s, %d, '%s')", ch.UUID, contact.ID, text))

	InsertIncomingMsg(c.rt, ch.OrgID, ch.ID, contact.ID, contact.URNID, text, dates.Now())

	return nil
}

func (c *MockCourier) ReportDelivered(ch *models.Channel, contact *models.Contact, msgID models.MsgID) error {
	c.Calls = append(c.Calls, fmt.Sprintf("ReportDelivered(%s, %d, %d)", ch.UUID, contact.ID, msgID))

	_, err := c.rt.DB.ExecContext(context.Background(), `UPDATE msgs_msg SET status = 'D', modified_on = NOW() WHERE id = $1 AND channel_id = $2`, msgID, ch.ID)
	noError(err)

	return nil
}
