package db

import (
	"context"
	"strings"
	"testing"

	"github.com/Zifeldev/emailback/service/internal/config"
)

func TestBuildDSN(t *testing.T) {
	dsn := BuildDSN(config.DatabaseConfig{Host: "localhost", Port: 5432, User: "u", Password: "p", Name: "n", SSLMode: "disable"})
	if !strings.HasPrefix(dsn, "postgres://") {
		t.Fatalf("dsn must start with postgres://, got %q", dsn)
	}
	if !strings.Contains(dsn, "localhost:5432") || !strings.Contains(dsn, "/n") || !strings.Contains(dsn, "sslmode=disable") {
		t.Fatalf("unexpected dsn: %q", dsn)
	}
}

func TestNew_InvalidConnection_FailsGracefully(t *testing.T) {
	cfg := config.DatabaseConfig{Host: "127.0.0.1", Port: 65432, User: "u", Password: "p", Name: "n", SSLMode: "disable"}
	if _, err := New(context.Background(), cfg); err == nil {
		t.Fatalf("expected error when connecting to invalid db")
	}
}
