package lang

import (
	"strings"
	"testing"
)

func TestCleanText_HTMLAndSignature_EN(t *testing.T) {
	raw := `<html><body><p>Hello <b>World</b>! Visit https://example.com</p><p>-- <br/>Best regards,<br/>John</p></body></html>`
	got := CleanText(raw)
	if got == "" {
		t.Fatalf("expected non-empty result for html")
	}
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Fatalf("expected no html tags, got: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "https://") || strings.Contains(strings.ToLower(got), "example.com") {
		t.Fatalf("expected urls removed, got: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "best regards") {
		t.Fatalf("expected signature removed, got: %q", got)
	}
}

func TestCleanText_QuotedAndSignature_RU(t *testing.T) {
	raw := "Привет,\n\n> Ответ на письмо\nСпасибо,\nС уважением\nИван"
	got := CleanText(raw)
	if strings.Contains(got, "> Ответ") {
		t.Fatalf("quoted text should be removed, got: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "с уважением") {
		t.Fatalf("signature should be removed, got: %q", got)
	}
	if !strings.Contains(got, "Привет") {
		t.Fatalf("expected greeting preserved, got: %q", got)
	}
}

func TestCleanText_GermanPatterns(t *testing.T) {
	raw := `Hallo Anna,

Am 1. Jan. 2025 schrieb Max Muster <max@example.com>:
> Zitatzeile
Mit freundlichen Grüßen
Max`
	got := CleanText(raw)
	if strings.Contains(got, "Zitatzeile") {
		t.Fatalf("quoted text should be removed for German, got: %q", got)
	}
	if strings.Contains(strings.ToLower(got), "mit freundlichen grüßen") {
		t.Fatalf("german signature should be removed, got: %q", got)
	}
	if !strings.Contains(got, "Hallo Anna") {
		t.Fatalf("greeting expected, got: %q", got)
	}
}

func TestCleanText_EmailAndURLRemoved(t *testing.T) {
	raw := "Contact: me@example.com or http://test.local/page\nThanks"
	got := CleanText(raw)
	if strings.Contains(got, "example.com") || strings.Contains(got, "http://") {
		t.Fatalf("expected emails and urls removed, got: %q", got)
	}
}
