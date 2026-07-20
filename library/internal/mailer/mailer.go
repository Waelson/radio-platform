// Package mailer provides SMTP e-mail delivery for the Library Service.
// Uses only the standard library net/smtp — no external dependency.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"

	"github.com/Waelson/radio-library-service/internal/config"
)

// Mailer sends transactional e-mails via SMTP.
type Mailer struct {
	cfg config.MailerConfig
	log *slog.Logger
}

// New creates a Mailer. When cfg.Enabled is false, messages are logged but not sent.
func New(cfg config.MailerConfig, log *slog.Logger) *Mailer {
	return &Mailer{cfg: cfg, log: log}
}

// SendResetCode delivers a 6-digit verification code to the recipient.
func (m *Mailer) SendResetCode(to, code string) error {
	subject := "Código de verificação — RadioFlow"
	body := fmt.Sprintf(
		"Seu código de verificação para redefinição de senha é:\n\n    %s\n\n"+
			"Este código é válido por 15 minutos e pode ser usado apenas uma vez.\n"+
			"Máximo de 5 tentativas por código.\n\n"+
			"Se você não solicitou este código, ignore este e-mail.\n",
		code,
	)
	return m.send(to, subject, body)
}

func (m *Mailer) send(to, subject, body string) error {
	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		m.cfg.From, to, subject, body,
	)

	if !m.cfg.Enabled {
		m.log.Info("mailer: (disabled) would send e-mail",
			"to", to, "subject", subject, "body", body)
		return nil
	}

	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	var auth smtp.Auth
	if m.cfg.Username != "" {
		auth = smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)
	}

	if m.cfg.UseTLS {
		return m.sendTLS(addr, auth, to, []byte(msg))
	}
	return smtp.SendMail(addr, auth, m.cfg.From, []string{to}, []byte(msg))
}

func (m *Mailer) sendTLS(addr string, auth smtp.Auth, to string, msg []byte) error {
	host, _, _ := net.SplitHostPort(addr)
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: host})
	if err != nil {
		return fmt.Errorf("mailer: tls dial %s: %w", addr, err)
	}
	client, err := smtp.NewClient(conn, host)
	if err != nil {
		return fmt.Errorf("mailer: new client: %w", err)
	}
	defer client.Quit() //nolint:errcheck
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: auth: %w", err)
		}
	}
	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	defer wc.Close() //nolint:errcheck
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("mailer: write body: %w", err)
	}
	return nil
}
