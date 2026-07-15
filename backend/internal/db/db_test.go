package db

import (
	"testing"
	"time"
)

func TestParseConnMaxIdleTime_Unset(t *testing.T) {
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "")

	d, ok := parseConnMaxIdleTime()
	if ok {
		t.Fatalf("expected ok=false when DB_CONN_MAX_IDLE_TIME is unset, got ok=true, d=%v", d)
	}
}

func TestParseConnMaxIdleTime_Valid(t *testing.T) {
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "1m")

	d, ok := parseConnMaxIdleTime()
	if !ok {
		t.Fatal("expected ok=true for valid duration \"1m\"")
	}
	if d != time.Minute {
		t.Errorf("expected d=%v, got %v", time.Minute, d)
	}
}

func TestParseConnMaxIdleTime_Invalid(t *testing.T) {
	t.Setenv("DB_CONN_MAX_IDLE_TIME", "not-a-duration")

	d, ok := parseConnMaxIdleTime()
	if ok {
		t.Fatalf("expected ok=false for unparseable duration, got ok=true, d=%v", d)
	}
}
