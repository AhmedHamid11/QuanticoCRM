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
// Returns the Gmail messageID, threadID, and any error.
func (p *GmailProvider) Send(ctx context.Context, orgID, userID, fromEmail, toEmail, subject, bodyHTML string) (messageID, threadID string, err error) {
	client, _, clientErr := p.oauthSvc.GetHTTPClient(ctx, orgID, userID)
	if clientErr != nil {
		return "", "", fmt.Errorf("gmail send: failed to get authenticated client: %w", clientErr)
	}

	rawMsg := buildRFC2822(fromEmail, toEmail, subject, bodyHTML)
	encoded := p.encodeMessage(rawMsg)

	payload, marshalErr := json.Marshal(map[string]string{"raw": encoded})
	if marshalErr != nil {
		return "", "", fmt.Errorf("gmail send: failed to marshal request payload: %w", marshalErr)
	}

	resp, postErr := client.Post(
		"https://gmail.googleapis.com/gmail/v1/users/me/messages/send",
		"application/json",
		bytes.NewReader(payload),
	)
	if postErr != nil {
		return "", "", fmt.Errorf("gmail send: HTTP request failed: %w", postErr)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("gmail send: API returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the response to extract id and threadId
	var result struct {
		ID       string `json:"id"`
		ThreadID string `json:"threadId"`
	}
	if parseErr := json.Unmarshal(body, &result); parseErr != nil {
		// Non-fatal: send succeeded but we can't extract IDs
		return "", "", nil
	}

	return result.ID, result.ThreadID, nil
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
