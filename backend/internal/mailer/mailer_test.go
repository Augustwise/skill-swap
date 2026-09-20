package mailer

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"skillswap/backend/internal/config"
)

func TestSMTPSendsTextMessage(t *testing.T) {
	address, received, stop := startSMTPServer(t)
	defer stop()
	client := NewSMTP(config.SMTPConfig{
		Addr: address,
		From: "Skill Swap <no-reply@students.example.test>",
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Send(ctx, Message{
		To:       []string{"student@example.test"},
		Subject:  "SMTP test",
		TextBody: "Mail delivery works.",
	}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	select {
	case message := <-received:
		for _, text := range []string{
			"From: \"Skill Swap\" <no-reply@students.example.test>",
			"To: <student@example.test>",
			"Subject: SMTP test",
			"Mail delivery works.",
		} {
			if !strings.Contains(message, text) {
				t.Fatalf("message does not contain %q:\n%s", text, message)
			}
		}
	case <-ctx.Done():
		t.Fatal("SMTP server did not receive the message")
	}
}

func TestSMTPRejectsHeaderInjection(t *testing.T) {
	client := NewSMTP(config.SMTPConfig{Addr: "127.0.0.1:1", From: "sender@example.test"})
	err := client.Send(context.Background(), Message{
		To:      []string{"student@example.test"},
		Subject: "Hello\r\nBcc: attacker@example.test",
	})
	if err == nil || !strings.Contains(err.Error(), "subject") {
		t.Fatalf("Send() error = %v", err)
	}
}

func startSMTPServer(t *testing.T) (string, <-chan string, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan string, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		writer := bufio.NewWriter(connection)
		writeSMTPLine(writer, "220 localhost ESMTP")
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			command := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(command, "EHLO"):
				writeSMTPLine(writer, "250 localhost")
			case strings.HasPrefix(command, "MAIL FROM:"), strings.HasPrefix(command, "RCPT TO:"):
				writeSMTPLine(writer, "250 OK")
			case command == "DATA":
				writeSMTPLine(writer, "354 End data with <CR><LF>.<CR><LF>")
				var message strings.Builder
				for {
					dataLine, err := reader.ReadString('\n')
					if err != nil {
						return
					}
					if dataLine == ".\r\n" {
						break
					}
					message.WriteString(dataLine)
				}
				received <- message.String()
				writeSMTPLine(writer, "250 Queued")
			case command == "QUIT":
				writeSMTPLine(writer, "221 Bye")
				return
			default:
				writeSMTPLine(writer, "502 Command not implemented")
			}
		}
	}()
	stop := func() {
		_ = listener.Close()
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Error("SMTP test server did not stop")
		}
	}
	return listener.Addr().String(), received, stop
}

func writeSMTPLine(writer *bufio.Writer, line string) {
	_, _ = fmt.Fprintf(writer, "%s\r\n", line)
	_ = writer.Flush()
}
