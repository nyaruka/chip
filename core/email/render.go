package email

import (
	"bytes"
	"embed"
	htmltemplate "html/template"
	"path"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/nyaruka/chip/v26/core/models"
)

//go:embed templates
var templatesFS embed.FS

var (
	textTemplate = texttemplate.Must(texttemplate.ParseFS(templatesFS, "templates/msgs.txt"))
	htmlTemplate = htmltemplate.Must(htmltemplate.ParseFS(templatesFS, "templates/msgs.html"))
)

// the context the templates render
type msgsContext struct {
	OrgName string
	Msgs    []*msgContext
}

type msgContext struct {
	Sender       string
	Time         string
	Lines        []string // the text, by line, so the HTML version can keep its line breaks while escaping it
	Attachments  []*attachmentContext
	QuickReplies string
}

type attachmentContext struct {
	URL     string
	Name    string
	IsImage bool
}

// Render renders an email to the given address of the given messages from the given org. Times are shown in the
// org's timezone, as the platform shows them.
func Render(to, orgName string, tz *time.Location, msgs []*models.ChatMsg) (*Email, error) {
	ctx := &msgsContext{OrgName: orgName, Msgs: make([]*msgContext, len(msgs))}

	for i, m := range msgs {
		// a message without a user was sent by the workspace, e.g. by a flow
		sender := orgName
		if user := m.Sender(); user != nil && user.Name() != "" {
			sender = user.Name()
		}

		mc := &msgContext{Sender: sender, Time: m.CreatedOn.In(tz).Format("Jan 2, 15:04")}
		if m.Text != "" {
			mc.Lines = strings.Split(m.Text, "\n")
		}
		for _, att := range m.Attachments {
			contentType, url, _ := strings.Cut(att, ":")
			mc.Attachments = append(mc.Attachments, &attachmentContext{
				URL:     url,
				Name:    path.Base(url),
				IsImage: strings.HasPrefix(contentType, "image/"),
			})
		}

		// quick replies can't be answered by email, so they're only shown as the options the visitor had
		var options []string
		for _, qr := range models.TextQuickReplies(m.QuickReplies) {
			options = append(options, qr.Text)
		}
		mc.QuickReplies = strings.Join(options, " · ")

		ctx.Msgs[i] = mc
	}

	subject := "New message from " + orgName
	if len(msgs) > 1 {
		subject = "New messages from " + orgName
	}

	var text, html bytes.Buffer
	if err := textTemplate.Execute(&text, ctx); err != nil {
		return nil, err
	}
	if err := htmlTemplate.Execute(&html, ctx); err != nil {
		return nil, err
	}

	return &Email{To: to, Subject: subject, Text: text.String(), HTML: html.String()}, nil
}
