// Package mail sends the API's emails: SMTP in prod, and the log in dev.
package mail

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

// Sender sends one plain-text email. The sign-in handlers use it for the email links.
type Sender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// SMTP sends through an SMTP relay. Port 465 uses TLS from the start; other ports must offer STARTTLS.
type SMTP struct {
	Host, Username, Password, From string
	Port                           int
}

// Send writes the message to the relay. It refuses to send the password over a connection without TLS.
func (m SMTP) Send(ctx context.Context, to, subject, body string) error {
	msg, err := Message(m.From, to, subject, body)
	if err != nil {
		return err
	}
	addr := net.JoinHostPort(m.Host, strconv.Itoa(m.Port))
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	if m.Port == 465 {
		conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12})
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("smtp dial: %w", err)
	}
	if dl, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(dl)
	} else {
		_ = conn.SetDeadline(time.Now().Add(30 * time.Second))
	}
	c, err := smtp.NewClient(conn, m.Host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("smtp: %w", err)
	}
	defer c.Close()
	if m.Port != 465 {
		if ok, _ := c.Extension("STARTTLS"); !ok {
			return errors.New("smtp: the relay does not offer STARTTLS")
		}
		if err := c.StartTLS(&tls.Config{ServerName: m.Host, MinVersion: tls.VersionTLS12}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	// PlainAuth itself refuses to send the password when the connection has no TLS.
	if err := c.Auth(smtp.PlainAuth("", m.Username, m.Password, m.Host)); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := c.Mail(m.From); err != nil {
		return fmt.Errorf("smtp from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp to: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	return c.Quit()
}

// Log writes each email to the log instead of sending it. main uses it only when ENV is dev.
type Log struct {
	L *slog.Logger
}

// Send writes the email to the log, so a developer can open a sign-in link without a mail server.
func (m Log) Send(_ context.Context, to, subject, body string) error {
	m.L.Info("dev email (not sent)", "to", to, "subject", subject, "body", body)
	return nil
}

// Message makes a plain-text email in the SMTP format. It refuses a line break in a header, which would let
// a caller add headers of their own. SMTP.Send calls it.
func Message(from, to, subject, body string) ([]byte, error) {
	for _, h := range []string{from, to, subject} {
		if strings.ContainsAny(h, "\r\n") {
			return nil, errors.New("mail: a header must not contain a line break")
		}
	}
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("Date: " + time.Now().UTC().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	// SMTP needs CRLF line ends, and a line that starts with "." would end the message early.
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		if strings.HasPrefix(line, ".") {
			line = "." + line
		}
		b.WriteString(line + "\r\n")
	}
	return []byte(b.String()), nil
}
