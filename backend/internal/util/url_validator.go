package util

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

// SECURITY: Block list of IP ranges that should never be accessed via webhooks
// This prevents SSRF attacks against internal infrastructure

// IsAllowedWebhookURL validates a URL is safe for webhook delivery
// Returns an error if the URL is blocked for security reasons
func IsAllowedWebhookURL(rawURL string) error {
	if rawURL == "" {
		return fmt.Errorf("webhook URL is required")
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http and https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("webhook URL must use http or https scheme")
	}

	// In production, require HTTPS
	if IsProduction() && parsed.Scheme != "https" {
		// Allow HTTP only if explicitly configured (for testing)
		if os.Getenv("ALLOW_HTTP_WEBHOOKS") != "true" {
			return fmt.Errorf("webhook URL must use https in production")
		}
	}

	hostname := parsed.Hostname()
	if hostname == "" {
		return fmt.Errorf("webhook URL must have a hostname")
	}

	// Block localhost and loopback addresses
	if isLocalhost(hostname) {
		return fmt.Errorf("webhook URL cannot target localhost")
	}

	// Block cloud metadata endpoints
	if isCloudMetadata(hostname) {
		return fmt.Errorf("webhook URL cannot target cloud metadata endpoints")
	}

	// Resolve hostname and check if it resolves to a private IP
	ips, err := net.LookupIP(hostname)
	if err != nil {
		// If we can't resolve, allow it (might be a valid external host)
		// The request will fail naturally if it's unreachable
		return nil
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("webhook URL resolves to private IP address (%s)", ip.String())
		}
	}

	return nil
}

// isLocalhost checks if hostname is localhost
func isLocalhost(hostname string) bool {
	lower := strings.ToLower(hostname)
	return lower == "localhost" ||
		lower == "127.0.0.1" ||
		lower == "::1" ||
		lower == "[::1]" ||
		lower == "0.0.0.0" ||
		strings.HasSuffix(lower, ".localhost") ||
		strings.HasSuffix(lower, ".local")
}

// isCloudMetadata checks if hostname is a cloud metadata endpoint
func isCloudMetadata(hostname string) bool {
	metadataHosts := []string{
		"169.254.169.254",         // AWS, GCP, Azure metadata
		"metadata.google.internal", // GCP
		"metadata.goog",           // GCP
		"169.254.170.2",           // AWS ECS task metadata
		"fd00:ec2::254",           // AWS IPv6 metadata
	}

	lower := strings.ToLower(hostname)
	for _, blocked := range metadataHosts {
		if lower == blocked {
			return true
		}
	}
	return false
}

// isPrivateIP checks if an IP address is in a private/reserved range
func isPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}

	// Check for loopback
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private ranges
	if ip.IsPrivate() {
		return true
	}

	// Check for unspecified (0.0.0.0 or ::)
	if ip.IsUnspecified() {
		return true
	}

	// Additional checks for IPv4 mapped IPv6 addresses
	if ip4 := ip.To4(); ip4 != nil {
		// 127.x.x.x (loopback)
		if ip4[0] == 127 {
			return true
		}
		// 169.254.x.x (link-local)
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
		// 10.x.x.x
		if ip4[0] == 10 {
			return true
		}
		// 172.16.x.x - 172.31.x.x
		if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
			return true
		}
		// 192.168.x.x
		if ip4[0] == 192 && ip4[1] == 168 {
			return true
		}
	}

	return false
}

// ValidateWebhookURLs validates a list of URLs
func ValidateWebhookURLs(urls []string) []string {
	var errors []string
	for _, u := range urls {
		if err := IsAllowedWebhookURL(u); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %s", u, err.Error()))
		}
	}
	return errors
}
