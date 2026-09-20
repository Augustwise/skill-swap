package mailer

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	"skillswap/backend/internal/config"
)

type Message struct {
	To       []string
	Subject  string
	TextBody string
}

type IMailer interface {
	Send(context.Context, Message) error
}

type SMTP struct {
	config config.SMTPConfig
}

var _ IMailer = (*SMTP)(nil)

func NewSMTP(cfg config.SMTPConfig) *SMTP {
	return &SMTP{config: cfg}
}

func (m *SMTP) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	from, err := parseAddress(m.config.From)
	if err != nil {
		return fmt.Errorf("invalid sender: %w", err)
	}
	if len(message.To) == 0 {
		return errors.New("at least one recipient is required")
	}
	if containsNewline(message.Subject) {
		return errors.New("subject must not contain a newline")
	}
	recipients := make([]*mail.Address, 0, len(message.To))
	for _, raw := range message.To {
		recipient, err := parseAddress(raw)
		if err != nil {
			return fmt.Errorf("invalid recipient: %w", err)
		}
		recipients = append(recipients, recipient)
	}
	payload, err := messageBytes(from, recipients, message)
	if err != nil {
		return err
	}

	host, _, err := net.SplitHostPort(m.config.Addr)
	if err != nil {
		return fmt.Errorf("invalid SMTP address: %w", err)
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", m.config.Addr)
	if err != nil {
		return fmt.Errorf("connect to SMTP server: %w", err)
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := connection.SetDeadline(deadline); err != nil {
			return fmt.Errorf("set SMTP deadline: %w", err)
		}
	}
	stopContextWatch := context.AfterFunc(ctx, func() {
		_ = connection.SetDeadline(time.Now())
	})
	defer stopContextWatch()
	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("start SMTP session: %w", err)
	}
	defer client.Close()
	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("start SMTP TLS: %w", err)
		}
	}
	if m.config.Username != "" {
		auth := smtp.PlainAuth("", m.config.Username, m.config.Password, host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("authenticate with SMTP server: %w", err)
		}
	}
	if err := client.Mail(from.Address); err != nil {
		return fmt.Errorf("set SMTP sender: %w", err)
	}
	for _, recipient := range recipients {
		if err := client.Rcpt(recipient.Address); err != nil {
			return fmt.Errorf("set SMTP recipient: %w", err)
		}
	}
	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("start SMTP message: %w", err)
	}
	buffered := bufio.NewWriter(writer)
	if _, err := buffered.Write(payload); err != nil {
		_ = writer.Close()
		return fmt.Errorf("write SMTP message: %w", err)
	}
	if err := buffered.Flush(); err != nil {
		_ = writer.Close()
		return fmt.Errorf("flush SMTP message: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("finish SMTP message: %w", err)
	}
	if err := client.Quit(); err != nil {
		return fmt.Errorf("close SMTP session: %w", err)
	}
	return nil
}

func messageBytes(from *mail.Address, recipients []*mail.Address, message Message) ([]byte, error) {
	to := make([]string, 0, len(recipients))
	for _, recipient := range recipients {
		to = append(to, recipient.String())
	}
	var builder strings.Builder
	fmt.Fprintf(&builder, "From: %s\r\n", from.String())
	fmt.Fprintf(&builder, "To: %s\r\n", strings.Join(to, ", "))
	fmt.Fprintf(&builder, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", message.Subject))
	fmt.Fprintf(&builder, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	builder.WriteString("MIME-Version: 1.0\r\n")
	builder.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	builder.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	quoted := quotedprintable.NewWriter(&builder)
	body := strings.ReplaceAll(strings.ReplaceAll(message.TextBody, "\r\n", "\n"), "\r", "\n")
	body = strings.ReplaceAll(body, "\n", "\r\n")
	if _, err := quoted.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("encode message body: %w", err)
	}
	if err := quoted.Close(); err != nil {
		return nil, fmt.Errorf("encode message body: %w", err)
	}
	return []byte(builder.String()), nil
}

func parseAddress(raw string) (*mail.Address, error) {
	if containsNewline(raw) {
		return nil, errors.New("address must not contain a newline")
	}
	return mail.ParseAddress(raw)
}

func containsNewline(value string) bool {
	return strings.ContainsAny(value, "\r\n")
}
