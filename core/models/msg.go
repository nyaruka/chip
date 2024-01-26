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
	ChatID  string
	Email   string
	Text    string
	Origin  MsgOrigin
	User    User
}

func NewMsgOut(id MsgID, ch Channel, chatID, email string, origin MsgOrigin, u User) *MsgOut {
	return &MsgOut{ID: id, Channel: ch, ChatID: chatID, Email: email, Origin: origin, User: u}
}
