package mail

import (
	"strings"
	"testing"
)

func TestMessageRefusesHeaderInjection(t *testing.T) {
	for _, to := range []string{"a@example.test\r\nBcc: x@evil.example", "a@example.test\nBcc: x@evil.example"} {
		if _, err := Message("from@example.test", to, "Hi", "body"); err == nil {
			t.Errorf("to %q: want an error", to)
		}
	}
}

func TestMessageUsesCRLFAndEscapesDotLines(t *testing.T) {
	msg, err := Message("from@example.test", "to@example.test", "Sign in", "line one\n.dot line\nlast")
	if err != nil {
		t.Fatal(err)
	}
	s := string(msg)
	if !strings.Contains(s, "Subject: Sign in\r\n") || !strings.Contains(s, "\r\n\r\nline one\r\n..dot line\r\nlast\r\n") {
		t.Errorf("message = %q", s)
	}
	if strings.Contains(strings.ReplaceAll(s, "\r\n", ""), "\n") {
		t.Error("every line must end with CRLF")
	}
}
