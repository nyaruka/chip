package courier

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nyaruka/gocommon/httpx"
	"github.com/nyaruka/gocommon/jsonx"
	"github.com/nyaruka/tembachat/runtime"
	"github.com/nyaruka/tembachat/webchat"
)

type courierChat struct {
	Identifier string `json:"identifier"`
}

type courierMsg struct {
	Identifier string `json:"identifier"`
	Text       string `json:"text"`
}

type courierPayload struct {
	Type string       `json:"type"`
	Chat *courierChat `json:"chat"`
	Msg  *courierMsg  `json:"msg"`
}

func notifyCourier(baseURL string, channelUUID webchat.ChannelUUID, payload *courierPayload) {
	url := fmt.Sprintf("%s/c/twc/%s/receive", baseURL, channelUUID)
	request, _ := httpx.NewRequest("POST", url, bytes.NewReader(jsonx.MustMarshal(payload)), nil)

	resp, err := httpx.Do(http.DefaultClient, request, nil, nil)
	if err != nil {
		slog.Error("error connecting to courier", "error", err)
	} else {
		slog.Info("courier notified", "event", payload.Type, "status", resp.StatusCode)
	}
}

func Notify(cfg *runtime.Config, c webchat.Client, e webchat.Event) {
	switch typed := e.(type) {
	case *webchat.ChatStartedEvent:
		notifyCourier(cfg.Courier, c.Channel().UUID(), &courierPayload{
			Type: "chat_started",
			Chat: &courierChat{
				Identifier: c.Identifier(),
			},
		})

	case *webchat.MsgInEvent:
		notifyCourier(cfg.Courier, c.Channel().UUID(), &courierPayload{
			Type: "msg_in",
			Msg: &courierMsg{
				Identifier: c.Identifier(),
				Text:       typed.Text,
			},
		})
	}

}
