package main

import (
	"testing"
)

func TestDatabaseURLUsesExplicitValue(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://explicit")
	t.Setenv("DB_PASSWORD", "ignored")

	if got := databaseURL(); got != "postgres://explicit" {
		t.Fatalf("databaseURL() = %q, want explicit DATABASE_URL", got)
	}
}

func TestDatabaseURLBuildsFromDiscreteEnv(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("DB_HOST", "db.example.internal")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "pulsecart")
	t.Setenv("DB_USER", "pulsecart")
	t.Setenv("DB_PASSWORD", "pa:ss/word?")
	t.Setenv("DB_SSLMODE", "require")

	want := "postgres://pulsecart:pa%3Ass%2Fword%3F@db.example.internal:5432/pulsecart?sslmode=require"
	if got := databaseURL(); got != want {
		t.Fatalf("databaseURL() = %q, want %q", got, want)
	}
}
