package web

import (
	"time"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/runtime"
)

// the types of the events a chat client sees. Chat events are their own client-centric vocabulary rather than the
// platform's engine events - from the widget's side an outgoing message is one it receives - though the content
// fields keep the same shapes as the engine's msg events, so widget code can be shared with the platform's own
// history views.
const (
	eventTypeMsgOut        = "msg_out"
	eventTypeMsgIn         = "msg_in"
	eventTypeTypingStarted = "typing_started"
	eventTypeTypingStopped = "typing_stopped"
)

// msgEvent is how a chat client sees a message: published to the conversation's chat socket as each outgoing
// message is sent, and returned by the history endpoint for messages in both directions - one shape for the
// client to parse whether a message arrives live or is fetched later
type msgEvent struct {
	Type         string              `json:"type"`
	CreatedOn    time.Time           `json:"created_on"`
	MsgUUID      models.MsgUUID      `json:"msg_uuid"`
	Text         string              `json:"text"`
	Attachments  []string            `json:"attachments,omitempty"`
	QuickReplies []models.QuickReply `json:"quick_replies,omitempty"`

	// who sent an outgoing message, when it was a user rather than a flow - so the chat can show them
	User *msgUser `json:"user,omitempty"`
}

// msgUser is a message's sender as the chat shows them - the same shape as the user references on the platform's
// own contact history events, so the widget renders them the way the app does
type msgUser struct {
	UUID   models.UserUUID `json:"uuid"   validate:"required"`
	Name   string          `json:"name"   validate:"required"`
	Avatar string          `json:"avatar,omitempty"`
}

// newMsgUser converts a user to how a chat event shows them, or nil for no user
func newMsgUser(rt *runtime.Runtime, user *models.User) *msgUser {
	if user == nil {
		return nil
	}
	return &msgUser{UUID: user.UUID(), Name: user.Name(), Avatar: user.AvatarURL(rt)}
}

// typingEvent is how a chat client sees a user composing a reply. The client decays the indicator on its own, so a
// lost typing_stopped only leaves it showing until then.
type typingEvent struct {
	Type      string    `json:"type"`
	CreatedOn time.Time `json:"created_on"`

	// who is typing - without an avatar since the platform only references users by UUID and name, which a client
	// can match against the senders of messages it has seen
	User *msgUser `json:"user,omitempty"`
}
