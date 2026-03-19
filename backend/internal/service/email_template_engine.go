package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/fastcrm/backend/internal/entity"
)

// ComplianceFooterToken is the template token that must appear in every commercial email body.
// Templates missing this token will fail validation at save time and render time.
const ComplianceFooterToken = "{{compliance_footer}}"

// TemplateEngine is a stateless regex-based email template renderer.
// It is safe for concurrent use — all state is in compiled regexes held at construction time.
// TrackingService is optional — if nil, InjectTracking is a no-op passthrough.
type TemplateEngine struct {
	conditionalRe        *regexp.Regexp
	conditionalNoElseRe  *regexp.Regexp
	variableWithFallback *regexp.Regexp
	variableRe           *regexp.Regexp
	trackingService      *TrackingService
}

// NewTemplateEngine compiles all regex patterns once and returns a ready-to-use TemplateEngine.
// Patterns use (?s) flag so . matches newlines, enabling correct rendering of multiline HTML blocks.
func NewTemplateEngine() *TemplateEngine {
	return &TemplateEngine{
		// {{#if field == "value"}}...{{else}}...{{/if}}
		conditionalRe: regexp.MustCompile(`(?s)\{\{#if\s+(\w+)\s*==\s*"([^"]*)"\}\}(.*?)\{\{else\}\}(.*?)\{\{/if\}\}`),
		// {{#if field == "value"}}...{{/if}} (no else branch)
		conditionalNoElseRe: regexp.MustCompile(`(?s)\{\{#if\s+(\w+)\s*==\s*"([^"]*)"\}\}(.*?)\{\{/if\}\}`),
		// {{variable|fallback}}
		variableWithFallback: regexp.MustCompile(`\{\{(\w+)\|([^}]*)\}\}`),
		// {{variable}}
		variableRe: regexp.MustCompile(`\{\{(\w+)\}\}`),
	}
}

// Render processes a template string with the given variable map and returns the rendered output.
// Processing order:
//  1. Conditionals with else — resolved first to avoid double-parsing
//  2. Conditionals without else
//  3. Variables with fallback
//  4. Plain variables
func (e *TemplateEngine) Render(template string, vars map[string]string) string {
	result := template

	// Step 1: Resolve {{#if field == "value"}}...{{else}}...{{/if}}
	result = e.conditionalRe.ReplaceAllStringFunc(result, func(match string) string {
		groups := e.conditionalRe.FindStringSubmatch(match)
		if len(groups) < 5 {
			return match
		}
		field := groups[1]
		value := groups[2]
		trueBody := groups[3]
		falseBody := groups[4]
		if vars[field] == value {
			return trueBody
		}
		return falseBody
	})

	// Step 2: Resolve {{#if field == "value"}}...{{/if}} (no else)
	result = e.conditionalNoElseRe.ReplaceAllStringFunc(result, func(match string) string {
		groups := e.conditionalNoElseRe.FindStringSubmatch(match)
		if len(groups) < 4 {
			return match
		}
		field := groups[1]
		value := groups[2]
		trueBody := groups[3]
		if vars[field] == value {
			return trueBody
		}
		return ""
	})

	// Step 3: Resolve {{variable|fallback}}
	result = e.variableWithFallback.ReplaceAllStringFunc(result, func(match string) string {
		groups := e.variableWithFallback.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}
		key := groups[1]
		fallback := groups[2]
		if val, ok := vars[key]; ok && val != "" {
			return val
		}
		return fallback
	})

	// Step 4: Resolve {{variable}}
	result = e.variableRe.ReplaceAllStringFunc(result, func(match string) string {
		groups := e.variableRe.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		key := groups[1]
		return vars[key]
	})

	return result
}

// ValidateCompliance checks that bodyHTML contains the {{compliance_footer}} token.
// Returns an error if the token is missing — this enforces CAN-SPAM compliance.
func (e *TemplateEngine) ValidateCompliance(bodyHTML string) error {
	if !strings.Contains(bodyHTML, ComplianceFooterToken) {
		return errors.New("template must include {{compliance_footer}} token for CAN-SPAM compliance")
	}
	return nil
}

// RenderTemplate renders both Subject and BodyHTML of an EmailTemplate using the given vars.
// Returns (subject, bodyHTML, error). An error is returned if the template body does not
// contain the {{compliance_footer}} token (CAN-SPAM compliance enforcement).
// The compliance_footer variable must be present in vars with the footer HTML content.
func (e *TemplateEngine) RenderTemplate(tmpl *entity.EmailTemplate, vars map[string]string) (string, string, error) {
	if err := e.ValidateCompliance(tmpl.BodyHTML); err != nil {
		return "", "", err
	}
	subject := e.Render(tmpl.Subject, vars)
	bodyHTML := e.Render(tmpl.BodyHTML, vars)
	return subject, bodyHTML, nil
}

// SetTrackingService attaches a TrackingService so InjectTracking can rewrite links and
// append the tracking pixel. Call this at startup after both services are constructed.
func (e *TemplateEngine) SetTrackingService(ts *TrackingService) {
	e.trackingService = ts
}

// InjectTracking rewrites all links in bodyHTML with tracking redirect URLs and appends
// a 1x1 tracking pixel. If no TrackingService is configured this is a passthrough.
func (e *TemplateEngine) InjectTracking(bodyHTML, orgID, enrollmentID, stepExecID string) string {
	if e.trackingService == nil {
		return bodyHTML
	}
	return e.trackingService.InjectTracking(bodyHTML, orgID, enrollmentID, stepExecID)
}

// ContactToTemplateVars converts a contact record (from the generic entity handler format,
// i.e. map[string]interface{}) to a template vars map[string]string.
// Nil or missing values become empty strings.
// The "company" key is sourced from the "account_name" field.
func (e *TemplateEngine) ContactToTemplateVars(contact map[string]interface{}) map[string]string {
	get := func(key string) string {
		v, ok := contact[key]
		if !ok || v == nil {
			return ""
		}
		return fmt.Sprintf("%v", v)
	}

	firstName := get("first_name")
	lastName := get("last_name")
	fullName := strings.TrimSpace(firstName + " " + lastName)

	return map[string]string{
		"first_name": firstName,
		"last_name":  lastName,
		"full_name":  fullName,
		"email":      get("email"),
		"phone":      get("phone"),
		"company":    get("account_name"),
		"title":      get("title"),
		"city":       get("city"),
		"state":      get("state"),
		"country":    get("country"),
	}
}
