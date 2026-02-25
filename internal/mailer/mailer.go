package mailer

import (
	"fmt"
	"time"

	"github.com/janiskrasemann/burrow/internal/renderer"
	"github.com/resend/resend-go/v3"
)

type Mailer struct {
	from        string
	to          string
	client      *resend.Client
	headerImage []byte
}

func New(from, to, apiKey string, headerImage []byte) *Mailer {
	return &Mailer{
		from:        from,
		to:          to,
		client:      resend.NewClient(apiKey),
		headerImage: headerImage,
	}
}

func (m *Mailer) Send(email *renderer.RenderedEmail) error {
	subject := fmt.Sprintf("Burrow Digest — %s", time.Now().Format("Jan 2, 2006"))

	params := &resend.SendEmailRequest{
		From:    m.from,
		To:      []string{m.to},
		Subject: subject,
		Html:    email.HTML,
		Text:    email.Text,
	}

	if len(m.headerImage) > 0 {
		params.Attachments = []*resend.Attachment{
			{
				Content:   m.headerImage,
				Filename:  "header.jpg",
				ContentId: "header-image",
			},
		}
	}

	sent, err := m.client.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("sending email via resend: %w", err)
	}

	fmt.Printf("email sent: %s\n", sent.Id)
	return nil
}
