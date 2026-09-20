// Command mailtest sends one development email through the configured SMTP server.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"skillswap/backend/internal/config"
	"skillswap/backend/internal/mailer"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: go run ./cmd/mailtest recipient@example.test")
	}
	cfg, err := config.LoadSMTP()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client := mailer.NewSMTP(cfg)
	if err := client.Send(ctx, mailer.Message{
		To:       []string{args[0]},
		Subject:  "Skill Swap: SMTP test",
		TextBody: "SMTP configuration is working. This is a development test email.",
	}); err != nil {
		return fmt.Errorf("send test email: %w", err)
	}
	fmt.Printf("Test email sent to %s via %s\n", args[0], cfg.Addr)
	return nil
}
