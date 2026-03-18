package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GmailProvider sends emails via the Gmail API using OAuth2 credentials
// managed by GmailOAuthService.
type GmailProvider struct {
	oauthSvc *GmailOAuthService
}

// NewGmailProvider creates a new GmailProvider backed by the given GmailOAuthService.
func NewGmailProvider(oauthSvc *GmailOAuthService) *GmailProvider {
	return &GmailProvider{oauthSvc: oauthSvc}
}

// Send composes an RFC 2822 email and delivers it via the Gmail API.
// orgID and userID identify the authenticated Gmail account to send from.
// fromEmail is used as the From header; toEmail is the recipient.
func (p *GmailProvider) Send(ctx context.Context, orgID, userID, fromEmail, toEmail, subject, bodyHTML string) error {
	client, _, err := p.oauthSvc.GetHTTPClient(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("gmail send: failed to get authenticated client: %w", err)
	}

	rawMsg := buildRFC2822(fromEmail, toEmail, subject, bodyHTML)
	encoded := p.encodeMessage(rawMsg)

	payload, err := json.Marshal(map[string]string{"raw": encoded})
	if err != nil {
		return fmt.Errorf("gmail send: failed to marshal request payload: %w", err)
	}

	resp, err := client.Post(
		"https://gmail.googleapis.com/gmail/v1/users/me/messages/send",
		"application/json",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("gmail send: HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("gmail send: API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// encodeMessage base64URL-encodes a raw RFC 2822 message for the Gmail API.
// The Gmail API requires base64url encoding (- and _ instead of + and /).
func (p *GmailProvider) encodeMessage(raw []byte) string {
	return base64.URLEncoding.EncodeToString(raw)
}

// buildRFC2822 constructs a minimal RFC 2822 compliant MIME message.
// All headers use \r\n line endings. A blank line separates headers from body.
func buildRFC2822(from, to, subject, bodyHTML string) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n")
	fmt.Fprintf(&buf, "\r\n")
	fmt.Fprintf(&buf, "%s", bodyHTML)
	return buf.Bytes()
}
