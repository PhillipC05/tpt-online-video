package auth

import (
	"context"
	"log"
)

// logSender implements EmailSender by logging emails to stdout.
// Useful for development and testing.
type logSender struct {
	config EmailConfig
}

func (s *logSender) SendPasswordResetEmail(ctx context.Context, to, resetURL string) error {
	log.Printf("[EMAIL] To: %s | Subject: Password Reset | Reset URL: %s", to, resetURL)
	return nil
}

func (s *logSender) SendEmailVerification(ctx context.Context, to, verifyURL string) error {
	log.Printf("[EMAIL] To: %s | Subject: Verify Email | Verify URL: %s", to, verifyURL)
	return nil
}

func (s *logSender) SendWelcomeEmail(ctx context.Context, to, displayName string) error {
	log.Printf("[EMAIL] To: %s | Subject: Welcome | Display Name: %s", to, displayName)
	return nil
}