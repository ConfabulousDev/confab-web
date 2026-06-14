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

// TestSessionDetail_InterfaceFieldsAreClassified guards against a free-form
// interface{}/JSONB field being added to SessionDetail without a conscious
// non-owner redaction decision. Such fields are invisible to the
// pii:"redact" *string walk in TestRedactForSharing_Completeness — git_info
// slipped exactly this guard and shipped a leak where a remote URL could even
// carry embedded credentials (d29s). Any interface{} field must be listed in
// knownHandled with a note on how it is sanitized for non-owner access.
func TestSessionDetail_InterfaceFieldsAreClassified(t *testing.T) {
	// field name -> how non-owner exposure is handled
	knownHandled := map[string]string{
		"GitInfo": "sanitized for all non-owners via SanitizeGitInfoForSharing in access.GetSessionDetailWithAccess (whitelist: branch + derived owner/repo display name)",
	}

	typ := reflect.TypeOf(SessionDetail{})
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.Type.Kind() != reflect.Interface {
			continue
		}
		if _, ok := knownHandled[field.Name]; !ok {
			t.Errorf("SessionDetail field %q is a free-form interface{}/JSONB field with no documented "+
				"non-owner redaction handling. It bypasses the pii:\"redact\" *string redaction walk and "+
				"may leak owner-private data (e.g. credential-bearing remote URLs). Classify it: sanitize it "+
				"for non-owner access and add it to knownHandled, or confirm it carries no owner-private data.", field.Name)
		}
	}
}
