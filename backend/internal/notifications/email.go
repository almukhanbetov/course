package notifications

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
)

type EmailMessage struct {
	To      string
	Subject string
	// HTMLBody is always full HTML — every template in templates.go
	// produces one, so there's no separate plain-text path to keep in sync.
	HTMLBody string
}

// EmailSender is the only thing the worker knows about how mail actually
// leaves the building. Swapping Mailpit for a real provider later (SES,
// Postmark, ...) means writing one new implementation of this interface —
// nothing in service.go/worker.go changes.
type EmailSender interface {
	Send(ctx context.Context, msg EmailMessage) error
}

// LogSender is the fallback when SMTP_HOST is unset — it never fails, so a
// development environment without Mailpit configured still exercises the
// full notification pipeline end to end, just without a real inbox at the
// other end.
type LogSender struct{}

func (LogSender) Send(_ context.Context, msg EmailMessage) error {
	slog.Info("notification-worker: dev email not sent, SMTP_HOST unconfigured", "service", "notification-worker", "to", msg.To, "subject", msg.Subject)
	return nil
}

type SMTPConfig struct {
	Host      string
	Port      string
	User      string
	Password  string
	FromEmail string
	FromName  string
}

type SMTPSender struct {
	cfg SMTPConfig
}

func NewSMTPSender(cfg SMTPConfig) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

// Send dials plainly rather than using smtp.SendMail, because SendMail
// requires a non-nil Auth — Mailpit (and plenty of internal relays) accept
// unauthenticated mail, which SendMail can't express. Auth is attempted
// only when SMTP_USER is actually configured.
func (s *SMTPSender) Send(_ context.Context, msg EmailMessage) error {
	addr := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer client.Close()

	if s.cfg.User != "" {
		auth := smtp.PlainAuth("", s.cfg.User, s.cfg.Password, s.cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}

	if err := client.Mail(s.cfg.FromEmail); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(msg.To); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s <%s>\r\n", s.cfg.FromName, s.cfg.FromEmail)
	fmt.Fprintf(&b, "To: %s\r\n", msg.To)
	fmt.Fprintf(&b, "Subject: %s\r\n", msg.Subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.HTMLBody)

	if _, err := wc.Write([]byte(b.String())); err != nil {
		wc.Close()
		return fmt.Errorf("write smtp body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("close smtp data: %w", err)
	}

	return client.Quit()
}
