package api

import (
	"testing"

	"github.com/ConfabulousDev/confab-web/internal/analytics"
)

func setSmartRecapEnv(t *testing.T, kv map[string]string) {
	t.Helper()
	for _, k := range []string{
		"SMART_RECAP_ENABLED", "SMART_RECAP_LLM_PROVIDER", "ANTHROPIC_API_KEY", "OPENAI_API_KEY",
		"SMART_RECAP_MODEL", "SMART_RECAP_QUOTA_LIMIT", "SMART_RECAP_MAX_OUTPUT_TOKENS",
		"SMART_RECAP_MAX_TRANSCRIPT_TOKENS", "TEST_SMART_RECAP_BASE_URL",
	} {
		t.Setenv(k, kv[k])
	}
}

func TestLoadSmartRecapConfig_DefaultsToAnthropic(t *testing.T) {
	setSmartRecapEnv(t, map[string]string{
		"SMART_RECAP_ENABLED": "true",
		"ANTHROPIC_API_KEY":   "ak",
		"SMART_RECAP_MODEL":   "claude-haiku-4-5-20251001",
	})

	cfg := loadSmartRecapConfig()

	if !cfg.Enabled {
		t.Error("Enabled: want true")
	}
	if cfg.Provider != analytics.LLMProviderAnthropic || cfg.APIKey != "ak" {
		t.Errorf("Provider/APIKey = %q/%q", cfg.Provider, cfg.APIKey)
	}
}

func TestLoadSmartRecapConfig_SelectsOpenAI(t *testing.T) {
	setSmartRecapEnv(t, map[string]string{
		"SMART_RECAP_ENABLED":       "true",
		"SMART_RECAP_LLM_PROVIDER":  "openai",
		"ANTHROPIC_API_KEY":         "ak",
		"OPENAI_API_KEY":            "ok",
		"SMART_RECAP_MODEL":         "gpt-5.6-luna",
		"TEST_SMART_RECAP_BASE_URL": "http://mock",
	})

	cfg := loadSmartRecapConfig()

	if !cfg.Enabled {
		t.Error("Enabled: want true")
	}
	if cfg.Provider != analytics.LLMProviderOpenAI || cfg.APIKey != "ok" {
		t.Errorf("Provider/APIKey = %q/%q, want openai/ok", cfg.Provider, cfg.APIKey)
	}

	gen := cfg.generatorConfig()
	if gen.Provider != analytics.LLMProviderOpenAI || gen.APIKey != "ok" || gen.Model != "gpt-5.6-luna" || gen.BaseURL != "http://mock" {
		t.Errorf("generatorConfig = %+v", gen)
	}
}

func TestLoadSmartRecapConfig_DisablesWhenActiveVendorKeyMissing(t *testing.T) {
	setSmartRecapEnv(t, map[string]string{
		"SMART_RECAP_ENABLED":      "true",
		"SMART_RECAP_LLM_PROVIDER": "openai",
		"ANTHROPIC_API_KEY":        "ak", // wrong vendor's key does not count
		"SMART_RECAP_MODEL":        "gpt-5.6-luna",
	})

	if loadSmartRecapConfig().Enabled {
		t.Error("Enabled: want false when OPENAI_API_KEY is missing for provider openai")
	}
}
