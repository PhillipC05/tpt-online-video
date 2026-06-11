package auth

import "context"

// EmailSender sends emails for auth flows (verification, password reset, etc.)
type EmailSender interface {
	// SendPasswordResetEmail sends a password reset email with the given token.
	SendPasswordResetEmail(ctx context.Context, to, resetURL string) error
	// SendEmailVerification sends an email verification email.
	SendEmailVerification(ctx context.Context, to, verifyURL string) error
	// SendWelcomeEmail sends a welcome email after successful registration.
	SendWelcomeEmail(ctx context.Context, to, displayName string) error
}

// EmailConfig configures the email provider.
type EmailConfig struct {
	FromName  string
	FromEmail string
	Provider  string // "smtp", "log", "sendgrid", "ses", etc.

	// SMTP settings
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	SMTPTLS      bool

	// App base URL for constructing links
	AppBaseURL string
}

// NewEmailSender creates an email sender based on the provider type.
// If provider is "log", it just logs emails instead of sending.
func NewEmailSender(cfg EmailConfig) EmailSender {
	switch cfg.Provider {
	case "smtp":
		return &smtpSender{config: cfg}
	case "log", "development", "":
		return &logSender{config: cfg}
	default:
		return &logSender{config: cfg}
	}
}