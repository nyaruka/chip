package web

import (
	"net/http"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/gocommon/dates"
)

// sendPayload is an outgoing message as courier hands it to us to deliver to the visitor. Courier has already
// resolved the sender - we only need how the chat shows them.
type sendPayload struct {
	ChannelUUID  models.ChannelUUID  `json:"channel_uuid"  validate:"required,uuid"`
	ChatID       string              `json:"chat_id"       validate:"required"`
	MsgUUID      models.MsgUUID      `json:"msg_uuid"      validate:"required,uuid"`
	Text         string              `json:"text"`
	Attachments  []string            `json:"attachments"`
	QuickReplies []models.QuickReply `json:"quick_replies" validate:"dive"`
	User         *msgUser            `json:"user"`

	// the contact's email address, if they have one, for emailing them messages they don't see in the chat
	Email string `json:"email"`
}

// handleSend is our handler for courier sending a message to a visitor: it's published to the conversation's chat
// socket as a msg_out event. Like all socket publishes this is presence-aware and best-effort - if the visitor
// doesn't currently have the chat open the publish is dropped, and the message is still considered sent. A failure
// to publish is an error, which courier treats as a failed send and retries.
func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	payload := &sendPayload{}
	if err := readJSON(w, r, payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := models.ChatURN(payload.ChatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	event := &msgEvent{
		Type:         eventTypeMsgOut,
		CreatedOn:    dates.Now(),
		MsgUUID:      payload.MsgUUID,
		Text:         payload.Text,
		Attachments:  payload.Attachments,
		QuickReplies: models.TextQuickReplies(payload.QuickReplies),
		User:         payload.User,
	}

	s.publish(w, r, models.ChatSocket(payload.ChannelUUID, payload.ChatID), event)
}

// eventPayload is something other than a message that courier has us show the visitor - a user typing
type eventPayload struct {
	ChannelUUID models.ChannelUUID `json:"channel_uuid" validate:"required,uuid"`
	ChatID      string             `json:"chat_id"      validate:"required"`
	Type        string             `json:"type"         validate:"required,oneof=typing_started typing_stopped"`
	User        *msgUser           `json:"user"`
}

// handleEvent is our handler for courier sending a visitor a user's typing, published to the chat socket like a
// message. Publishing is cheap so typing_stopped is carried too, and typing_started is resent often enough to
// stay ahead of a client decaying the indicator.
func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	payload := &eventPayload{}
	if err := readJSON(w, r, payload); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := models.ChatURN(payload.ChatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return
	}

	event := &typingEvent{Type: payload.Type, CreatedOn: dates.Now()}
	if payload.User != nil {
		event.User = &msgUser{UUID: payload.User.UUID, Name: payload.User.Name}
	}

	s.publish(w, r, models.ChatSocket(payload.ChannelUUID, payload.ChatID), event)
}

// publish publishes the given event to the given socket and answers the request accordingly
func (s *Server) publish(w http.ResponseWriter, r *http.Request, socket string, event any) {
	if err := s.rt.Centrifugo.Publish(r.Context(), &centrifugo.Publication{Channel: socket, Data: event}); err != nil {
		logRequestError(r, "error publishing event", err, "socket", socket)
		writeError(w, http.StatusInternalServerError, "error publishing event")
		return
	}

	writeJSON(w, http.StatusOK, struct{}{})
}
