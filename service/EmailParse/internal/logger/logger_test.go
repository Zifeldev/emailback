package logger

import (
	"os"
	"testing"
)

func TestNew_DefaultFormatter(t *testing.T) {
	_ = os.Unsetenv("ENV")
	l := New()
	if l.Logger == nil {
		t.Fatalf("expected logger created")
	}
}

func TestNew_JSONFormatter(t *testing.T) {
	_ = os.Setenv("ENV", "production")
	t.Cleanup(func() { _ = os.Unsetenv("ENV") })
	l := New()
	if l.Logger == nil {
		t.Fatalf("expected logger created")
	}
}
