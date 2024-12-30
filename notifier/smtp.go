package notifier

import (
	"crypto/tls"
	"io"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/0x2142/frigate-notify/config"
	"github.com/0x2142/frigate-notify/models"
	"github.com/wneessen/go-mail"
)

// SendSMTP forwards alert data via email
func SendSMTP(event models.Event, snapshot io.Reader, provider notifMeta) {
	profile := config.ConfigData.Alerts.SMTP[provider.index]
	status := &config.Internal.Status.Notifications.SMTP[provider.index]

	// Build notification
	var message string
	if profile.Template != "" {
		message = renderMessage(profile.Template, event, "message", "SMTP")
	} else {
		message = renderMessage("html", event, "message", "SMTP")
	}

	// Set up email alert
	m := mail.NewMsg()
	m.From(profile.From)
	m.To(ParseSMTPRecipients(profile.Recipient)...)
	title := renderMessage(config.ConfigData.Alerts.General.Title, event, "title", "SMTP")
	m.Subject(title)
	// Attach snapshot if one exists
	if event.HasSnapshot {
		m.AttachReader("snapshot.jpg", snapshot)
	}

	// Convert message body to HTML
	m.SetBodyString(mail.TypeTextHTML, message)

	time.Sleep(5 * time.Second)

	// Set up SMTP Connection
	c, err := mail.NewClient(profile.Server, mail.WithPort(profile.Port))
	// Add authentication params if needed
	if profile.User != "" && profile.Password != "" {
		c.SetSMTPAuth(mail.SMTPAuthPlain)
		c.SetUsername(profile.User)
		c.SetPassword(profile.Password)
	}
	// Mandatory TLS is enabled by default, so disable TLS if config flag is set
	if !profile.TLS {
		c.SetTLSPolicy(mail.NoTLS)
	}
	// Disable certificate verification if needed
	if profile.Insecure {
		c.SetTLSConfig(&tls.Config{InsecureSkipVerify: true})
	}

	if err != nil {
		log.Warn().
			Str("event_id", event.ID).
			Str("provider", "SMTP").
			Err(err).
			Int("provider_id", provider.index).
			Msg("Unable to send alert")
		status.NotifFailure(err.Error())
	}

	log.Trace().
		Strs("sender", m.GetFromString()).
		Strs("recipients", m.GetToString()).
		Str("subject", title).
		Interface("payload", message).
		Str("server", profile.Server).
		Int("port", profile.Port).
		Bool("tls", profile.TLS).
		Str("username", profile.User).
		Str("password", "--secret removed--").
		Int("provider_id", provider.index).
		Msg("Send SMTP Alert")

	// Send message
	if err := c.DialAndSend(m); err != nil {
		log.Warn().
			Str("event_id", event.ID).
			Str("provider", "SMTP").
			Int("provider_id", provider.index).
			Err(err).
			Msg("Unable to send alert")
		status.NotifFailure(err.Error())
		return
	}
	log.Info().
		Str("event_id", event.ID).
		Str("provider", "SMTP").
		Int("provider_id", provider.index).
		Msg("Alert sent")
	status.NotifSuccess()
}

// ParseSMTPRecipients splits individual email addresses from config file
func ParseSMTPRecipients(recipientList string) []string {
	var recipients []string
	list := strings.Split(recipientList, ",")
	for _, addr := range list {
		recipients = append(recipients, strings.TrimSpace(addr))
	}
	return recipients
}
