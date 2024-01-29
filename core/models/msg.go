package models

type MsgID int64

type MsgOrigin string

const (
	MsgOriginFlow      MsgOrigin = "flow"
	MsgOriginBroadcast MsgOrigin = "broadcast"
	MsgOriginTicket    MsgOrigin = "ticket"
	MsgOriginChat      MsgOrigin = "chat"
)

type MsgOut struct {
	ID      MsgID
	Channel Channel
	Contact Contact
	Text    string
	Origin  MsgOrigin
	User    User
}

func NewMsgOut(id MsgID, ch Channel, contact Contact, text string, origin MsgOrigin, u User) *MsgOut {
	return &MsgOut{ID: id, Channel: ch, Contact: contact, Text: text, Origin: origin, User: u}
}
