package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

type mockDetector struct {
	code string
	conf float64
	ok   bool
}

func (m mockDetector) Detect(text string) (string, float64, bool) {
	if strings.TrimSpace(text) == "" {
		return "", 0, false
	}
	if m.ok {
		return m.code, m.conf, true
	}
	return "", 0, false
}

func TestEnmimeParser_Parse_PlainText(t *testing.T) {
	raw := []byte(strings.ReplaceAll(`Subject: Test Email
From: "Alice" <alice@example.com>
To: bob@example.com, carol@example.com
Date: Mon, 02 Jan 2006 15:04:05 +0000
Message-ID: <message-123@example.com>
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

Hello world! This is a test.
`, "\n", "\r\n"))

	det := mockDetector{code: "en", conf: 0.99, ok: true}
	p := NewEnmimeParser(Options{IncludeHTML: false}, det)

	ent, err := p.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if ent == nil {
		t.Fatal("expected entity, got nil")
	}

	if ent.MessageID != "message-123@example.com" {
		t.Errorf("message id mismatch: %q", ent.MessageID)
	}
	if ent.From != "alice@example.com" {
		t.Errorf("from mismatch: %q", ent.From)
	}
	if len(ent.To) != 2 {
		t.Errorf("expected 2 recipients, got %d", len(ent.To))
	}
	if ent.Subject != "Test Email" {
		t.Errorf("subject mismatch: %q", ent.Subject)
	}
	if ent.Date == nil {
		t.Fatalf("expected parsed date, got nil")
	}
	wantDate := time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC)
	if !ent.Date.UTC().Equal(wantDate) {
		t.Errorf("date mismatch: got %v want %v", ent.Date.UTC(), wantDate)
	}
	if ent.HTML != "" {
		t.Errorf("expected no HTML when IncludeHTML=false, got len=%d", len(ent.HTML))
	}
	if ent.Language != "en" || ent.Confidence <= 0 {
		t.Errorf("unexpected language result: %q conf=%v", ent.Language, ent.Confidence)
	}
	if ent.Metrics["word_count"].(int) == 0 {
		t.Errorf("expected non-zero word_count")
	}
	if !strings.Contains(ent.Text, "Hello world!") {
		t.Errorf("clean text doesn't contain body, got: %q", ent.Text)
	}
}

func TestEnmimeParser_Parse_HTMLFallback(t *testing.T) {
	html := `<html><body><p>Hello <b>World</b></p></body></html>`
	raw := []byte(strings.ReplaceAll(`Subject: HTML Only
From: alice@example.com
To: bob@example.com
Date: Mon, 02 Jan 2006 15:04:05 +0000
Message-ID: <message-999@example.com>
MIME-Version: 1.0
Content-Type: text/html; charset=UTF-8

`+html+`
`, "\n", "\r\n"))

	det := mockDetector{code: "en", conf: 1.0, ok: true}
	p := NewEnmimeParser(Options{IncludeHTML: true}, det)

	ent, err := p.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if ent.HTML == "" {
		t.Errorf("expected HTML to be included")
	}
	if !(strings.Contains(ent.Text, "Hello World") || strings.Contains(ent.Text, "Hello *World")) {
		t.Errorf("expected cleaned text to contain 'Hello World' (or '*World'), got: %q", ent.Text)
	}
}

func TestEnmimeParser_Parse_NoMessageID_SubjectDecode_ToMulti_NoDate_Attachments(t *testing.T) {
	boundary := "mixedb"
	encodedSubj := "=?UTF-8?B?0J/RgNC40LLQtdGCLCDQv9C+0LvRjNC30LDRgNCw?=" 
	raw := []byte(strings.ReplaceAll(
		"From: Bob <bob@example.com>\n"+
			"To: alice@example.com\n"+
			"To: carol@example.com\n"+
			"Subject: "+encodedSubj+"\n"+
			"MIME-Version: 1.0\n"+
			"Content-Type: multipart/mixed; boundary=\""+boundary+"\"\n"+
			"\n"+
			"--"+boundary+"\n"+
			"Content-Type: text/plain; charset=UTF-8\n\n"+
			"Body line 1\nBody line 2\n"+
			"--"+boundary+"\n"+
			"Content-Type: application/octet-stream\n"+
			"Content-Disposition: attachment; filename=\"file.txt\"\n\n"+
			"filecontent"+"\n"+
			"--"+boundary+"--\n",
		"\n", "\r\n"))

	det := mockDetector{code: "ru", conf: 0.8, ok: true}
	p := NewEnmimeParser(Options{IncludeHTML: false}, det)
	ent, err := p.Parse(context.Background(), raw)
	if err != nil {
		t.Fatalf("parse err: %v", err)
	}
	if ent.MessageID == "" {
		t.Fatalf("expected generated MessageID")
	}
	if !strings.Contains(strings.ToLower(ent.Subject), "привет") {
		t.Fatalf("expected decoded subject, got %q", ent.Subject)
	}
	if len(ent.To) != 2 {
		t.Fatalf("expected 2 To, got %d", len(ent.To))
	}
	if ent.Date != nil {
		t.Fatalf("expected nil date when missing, got %v", ent.Date)
	}
	if ent.Metrics["attachments"].(int) < 1 {
		t.Fatalf("expected attachments metric >=1")
	}
	if ent.Metrics["line_count"].(int) < 2 {
		t.Fatalf("expected line_count >=2")
	}
}
