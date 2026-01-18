package analytics

import (
	"testing"
)

func TestParseSmartRecapResponse_WithSuggestedTitle(t *testing.T) {
	input := `{
		"suggested_session_title": "Implement dark mode feature",
		"recap": "User implemented dark mode with Claude's help.",
		"went_well": ["Clear requirements"],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "Implement dark mode feature" {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, "Implement dark mode feature")
	}
	if result.Recap != "User implemented dark mode with Claude's help." {
		t.Errorf("Recap = %q, want %q", result.Recap, "User implemented dark mode with Claude's help.")
	}
}

func TestParseSmartRecapResponse_TruncatesLongTitle(t *testing.T) {
	// Create a title that's over 100 characters
	longTitle := "This is a very long session title that exceeds the maximum allowed length of one hundred characters by quite a bit"
	input := `{
		"suggested_session_title": "` + longTitle + `",
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if len(result.SuggestedSessionTitle) > 100 {
		t.Errorf("SuggestedSessionTitle length = %d, want <= 100", len(result.SuggestedSessionTitle))
	}
	if result.SuggestedSessionTitle != longTitle[:100] {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, longTitle[:100])
	}
}

func TestParseSmartRecapResponse_EmptyTitle(t *testing.T) {
	input := `{
		"suggested_session_title": "",
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "" {
		t.Errorf("SuggestedSessionTitle = %q, want empty string", result.SuggestedSessionTitle)
	}
}

func TestParseSmartRecapResponse_MissingTitle(t *testing.T) {
	// Title field completely missing from JSON
	input := `{
		"recap": "Test recap",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	// Should be empty string (zero value)
	if result.SuggestedSessionTitle != "" {
		t.Errorf("SuggestedSessionTitle = %q, want empty string", result.SuggestedSessionTitle)
	}
}

func TestParseSmartRecapResponse_ExtractsJSONFromText(t *testing.T) {
	// Sometimes LLMs add text around the JSON
	input := `Here is the analysis:
	{
		"suggested_session_title": "Debug authentication flow",
		"recap": "Fixed auth bug",
		"went_well": [],
		"went_bad": [],
		"human_suggestions": [],
		"environment_suggestions": [],
		"default_context_suggestions": []
	}
	That's my analysis.`

	result, err := parseSmartRecapResponse(input)
	if err != nil {
		t.Fatalf("parseSmartRecapResponse failed: %v", err)
	}

	if result.SuggestedSessionTitle != "Debug authentication flow" {
		t.Errorf("SuggestedSessionTitle = %q, want %q", result.SuggestedSessionTitle, "Debug authentication flow")
	}
}
