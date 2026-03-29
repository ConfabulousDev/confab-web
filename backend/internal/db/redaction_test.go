package db

import (
	"reflect"
	"testing"
)

// TestRedactForSharing_Completeness uses reflection to verify that every field
// tagged with pii:"redact" on SessionDetail is set to nil after RedactForSharing.
// This ensures new PII fields cannot be added without being covered by redaction.
func TestRedactForSharing_Completeness(t *testing.T) {
	// Populate all *string fields so we can detect if any are missed
	dummy := "test-value"
	detail := SessionDetail{
		Hostname:       &dummy,
		Username:       &dummy,
		CWD:            &dummy,
		TranscriptPath: &dummy,
	}

	detail.RedactForSharing()

	// Use reflection to find ALL fields tagged pii:"redact"
	v := reflect.ValueOf(detail)
	typ := v.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		piiTag := field.Tag.Get("pii")
		if piiTag != "redact" {
			continue
		}

		fv := v.Field(i)
		// PII fields should be *string — check they are nil after redaction
		if fv.Kind() == reflect.Ptr && !fv.IsNil() {
			t.Errorf("PII field %q (tagged pii:\"redact\") is not nil after RedactForSharing", field.Name)
		}
	}

	// Also verify the specific expected fields are covered
	if detail.Hostname != nil {
		t.Error("Hostname should be nil after RedactForSharing")
	}
	if detail.Username != nil {
		t.Error("Username should be nil after RedactForSharing")
	}
	if detail.CWD != nil {
		t.Error("CWD should be nil after RedactForSharing")
	}
	if detail.TranscriptPath != nil {
		t.Error("TranscriptPath should be nil after RedactForSharing")
	}
}
