package service

import (
	"testing"
)

func TestTemplateRenderSimpleVariable(t *testing.T) {
	engine := NewTemplateEngine()
	result := engine.Render("Hello {{first_name}}", map[string]string{"first_name": "Alice"})
	if result != "Hello Alice" {
		t.Errorf("expected 'Hello Alice', got %q", result)
	}
}

func TestTemplateRenderMissingVariable(t *testing.T) {
	engine := NewTemplateEngine()
	result := engine.Render("Hello {{first_name}}", map[string]string{})
	if result != "Hello " {
		t.Errorf("expected 'Hello ', got %q", result)
	}
}

func TestTemplateRenderFallback(t *testing.T) {
	engine := NewTemplateEngine()
	result := engine.Render("Hello {{first_name|there}}", map[string]string{})
	if result != "Hello there" {
		t.Errorf("expected 'Hello there', got %q", result)
	}
}

func TestTemplateRenderFallbackNotUsedWhenPresent(t *testing.T) {
	engine := NewTemplateEngine()
	result := engine.Render("Hello {{first_name|there}}", map[string]string{"first_name": "Alice"})
	if result != "Hello Alice" {
		t.Errorf("expected 'Hello Alice', got %q", result)
	}
}

func TestTemplateRenderConditionalTrue(t *testing.T) {
	engine := NewTemplateEngine()
	template := `{{#if status == "active"}}Active!{{else}}Inactive{{/if}}`
	result := engine.Render(template, map[string]string{"status": "active"})
	if result != "Active!" {
		t.Errorf("expected 'Active!', got %q", result)
	}
}

func TestTemplateRenderConditionalFalse(t *testing.T) {
	engine := NewTemplateEngine()
	template := `{{#if status == "active"}}Active!{{else}}Inactive{{/if}}`
	result := engine.Render(template, map[string]string{"status": "paused"})
	if result != "Inactive" {
		t.Errorf("expected 'Inactive', got %q", result)
	}
}

func TestTemplateRenderConditionalNoElse(t *testing.T) {
	engine := NewTemplateEngine()
	template := `{{#if status == "active"}}Active!{{/if}}`
	result := engine.Render(template, map[string]string{"status": "paused"})
	if result != "" {
		t.Errorf("expected '', got %q", result)
	}
}

func TestTemplateRenderMultiline(t *testing.T) {
	engine := NewTemplateEngine()
	template := `{{#if show == "yes"}}<p>
  Hello World
</p>{{else}}<p>Hidden</p>{{/if}}`
	result := engine.Render(template, map[string]string{"show": "yes"})
	expected := "<p>\n  Hello World\n</p>"
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestTemplateRenderMultipleVariables(t *testing.T) {
	engine := NewTemplateEngine()
	subject := "Hi {{first_name}} from {{company}}"
	body := "<p>Dear {{first_name}} {{last_name}}, welcome to {{company}}.</p>"
	vars := map[string]string{
		"first_name": "Jane",
		"last_name":  "Doe",
		"company":    "Acme",
	}
	renderedSubject := engine.Render(subject, vars)
	renderedBody := engine.Render(body, vars)
	if renderedSubject != "Hi Jane from Acme" {
		t.Errorf("subject: expected 'Hi Jane from Acme', got %q", renderedSubject)
	}
	if renderedBody != "<p>Dear Jane Doe, welcome to Acme.</p>" {
		t.Errorf("body: expected '<p>Dear Jane Doe, welcome to Acme.</p>', got %q", renderedBody)
	}
}

func TestTemplateRenderNestedConditionalAndVariable(t *testing.T) {
	engine := NewTemplateEngine()
	template := `{{#if status == "active"}}Hello {{first_name}}!{{else}}Sorry {{first_name}}{{/if}}`
	result := engine.Render(template, map[string]string{"status": "active", "first_name": "Bob"})
	if result != "Hello Bob!" {
		t.Errorf("expected 'Hello Bob!', got %q", result)
	}
}

func TestContactToTemplateVars(t *testing.T) {
	engine := NewTemplateEngine()
	contact := map[string]interface{}{
		"first_name":   "Jane",
		"last_name":    "Doe",
		"email":        "jane@acme.com",
		"phone":        "555-1234",
		"account_name": "Acme Corp",
		"title":        "VP Sales",
		"city":         "New York",
		"state":        "NY",
		"country":      "USA",
	}

	vars := engine.ContactToTemplateVars(contact)

	tests := []struct {
		key      string
		expected string
	}{
		{"first_name", "Jane"},
		{"last_name", "Doe"},
		{"full_name", "Jane Doe"},
		{"email", "jane@acme.com"},
		{"phone", "555-1234"},
		{"company", "Acme Corp"},
		{"title", "VP Sales"},
		{"city", "New York"},
		{"state", "NY"},
		{"country", "USA"},
	}

	for _, tt := range tests {
		if vars[tt.key] != tt.expected {
			t.Errorf("ContactToTemplateVars[%q]: expected %q, got %q", tt.key, tt.expected, vars[tt.key])
		}
	}
}
