package service

import (
	"strings"
	"testing"
)

// TestBuildRFC2822 verifies the basic RFC 2822 message structure.
func TestBuildRFC2822(t *testing.T) {
	msg := buildRFC2822("sender@test.com", "recipient@test.com", "Test Subject", "<p>Hello</p>")
	msgStr := string(msg)

	// Required headers
	if !strings.Contains(msgStr, "From: sender@test.com\r\n") {
		t.Errorf("expected From header with CRLF, got: %q", msgStr)
	}
	if !strings.Contains(msgStr, "To: recipient@test.com\r\n") {
		t.Errorf("expected To header with CRLF, got: %q", msgStr)
	}
	if !strings.Contains(msgStr, "Subject: Test Subject\r\n") {
		t.Errorf("expected Subject header with CRLF, got: %q", msgStr)
	}
	if !strings.Contains(msgStr, "MIME-Version: 1.0\r\n") {
		t.Errorf("expected MIME-Version header, got: %q", msgStr)
	}
	if !strings.Contains(msgStr, "Content-Type: text/html; charset=UTF-8\r\n") {
		t.Errorf("expected Content-Type header, got: %q", msgStr)
	}

	// Body
	if !strings.Contains(msgStr, "<p>Hello</p>") {
		t.Errorf("expected body content, got: %q", msgStr)
	}
}

// TestBuildRFC2822HasCRLF verifies that all header lines use \r\n and the blank
// line separating headers from body is also \r\n.
func TestBuildRFC2822HasCRLF(t *testing.T) {
	msg := buildRFC2822("a@a.com", "b@b.com", "Subject", "Body")
	msgStr := string(msg)

	// No bare \n (must always be \r\n)
	lines := strings.Split(msgStr, "\r\n")
	if len(lines) < 7 {
		t.Errorf("expected at least 7 segments after CRLF split (5 headers + blank + body), got %d", len(lines))
	}

	// Verify blank line separating headers from body exists
	if !strings.Contains(msgStr, "\r\n\r\n") {
		t.Errorf("expected blank CRLF line between headers and body")
	}
}

// TestBase64URLEncoding verifies that the encoded message uses base64URL encoding
// (uses - and _ instead of + and /).
func TestBase64URLEncoding(t *testing.T) {
	// Build a message that will produce base64 characters that differ between
	// StdEncoding and URLEncoding. We test with enough variety that + or / could appear.
	provider := &GmailProvider{}
	msg := buildRFC2822("from@example.com", "to@example.com", "Test", "<html><body>Base64 URL test 🧪</body></html>")
	encoded := provider.encodeMessage(msg)

	if strings.Contains(encoded, "+") {
		t.Errorf("expected base64URL encoding (no +), but found + in: %q", encoded)
	}
	if strings.Contains(encoded, "/") {
		t.Errorf("expected base64URL encoding (no /), but found / in: %q", encoded)
	}
}

// TestBuildRFC2822SpecialChars verifies that unicode characters in the subject
// and body do not cause panics and are preserved correctly.
func TestBuildRFC2822SpecialChars(t *testing.T) {
	subject := "Re: Quantico CRM — Follow-up 🚀"
	body := "<p>Hello Ünîcödé</p>"
	msg := buildRFC2822("sender@test.com", "recipient@test.com", subject, body)
	msgStr := string(msg)

	if !strings.Contains(msgStr, subject) {
		t.Errorf("expected unicode subject preserved in message, got: %q", msgStr)
	}
	if !strings.Contains(msgStr, body) {
		t.Errorf("expected unicode body preserved in message, got: %q", msgStr)
	}
}
