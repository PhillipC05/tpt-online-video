package auth

import (
	"context"
	"fmt"
	"net/smtp"
)

// smtpSender implements EmailSender using SMTP.
type smtpSender struct {
	config EmailConfig
}

func (s *smtpSender) SendPasswordResetEmail(ctx context.Context, to, resetURL string) error {
	subject := "Password Reset"
	body := fmt.Sprintf(`Hello,

You requested a password reset for your account. Click the link below to reset your password:

%s

If you did not request this, please ignore this email.

Best regards,
%s`, resetURL, s.config.FromName)

	return s.send(ctx, to, subject, body)
}

func (s *smtpSender) SendEmailVerification(ctx context.Context, to, verifyURL string) error {
	subject := "Verify Your Email"
	body := fmt.Sprintf(`Hello,

Welcome! Please verify your email address by clicking the link below:

%s

Best regards,
%s`, verifyURL, s.config.FromName)

	return s.send(ctx, to, subject, body)
}

func (s *smtpSender) SendWelcomeEmail(ctx context.Context, to, displayName string) error {
	subject := "Welcome!"
	body := fmt.Sprintf(`Hello %s,

Welcome to %s! Your account has been created successfully.

Best regards,
%s`, displayName, s.config.FromName, s.config.FromName)

	return s.send(ctx, to, subject, body)
}

func (s *smtpSender) send(ctx context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	auth := smtp.PlainAuth("", s.config.SMTPUsername, s.config.SMTPPassword, s.config.SMTPHost)

	from := s.config.FromEmail
	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.config.FromName, from, to, subject, body)

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	return smtp.SendMail(addr, auth, from, []string{to}, []byte(msg))
}