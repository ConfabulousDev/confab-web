package auth

import (
	"strings"
	"testing"
)

func TestGenerateDevicePageHTML(t *testing.T) {
	t.Run("prefilled code", func(t *testing.T) {
		html := generateDevicePageHTML("ABCD-1234")
		if !strings.Contains(html, "Authorize Device") {
			t.Error("missing page title text")
		}
		if !strings.Contains(html, `value="ABCD-1234"`) {
			t.Error("prefilled code not embedded in input value attribute")
		}
		if !strings.Contains(html, `action="/auth/device/verify"`) {
			t.Error("form must POST to /auth/device/verify")
		}
	})

	t.Run("empty code", func(t *testing.T) {
		html := generateDevicePageHTML("")
		if !strings.Contains(html, `value=""`) {
			t.Error("empty code should produce empty value attribute")
		}
	})
}

func TestGenerateDeviceResultHTML(t *testing.T) {
	t.Run("success path", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "https://app.example.com")
		html := generateDeviceResultHTML(true, "all good")
		if !strings.Contains(html, "Success!") {
			t.Error("missing success title")
		}
		if !strings.Contains(html, "all good") {
			t.Error("missing message")
		}
		if !strings.Contains(html, "https://app.example.com") {
			t.Error("home link should be rendered when FRONTEND_URL set")
		}
	})

	t.Run("success path without FRONTEND_URL", func(t *testing.T) {
		t.Setenv("FRONTEND_URL", "")
		html := generateDeviceResultHTML(true, "ok")
		if strings.Contains(html, `class="home-link"`) {
			t.Error("home link should be omitted when FRONTEND_URL unset")
		}
	})

	t.Run("failure path", func(t *testing.T) {
		html := generateDeviceResultHTML(false, "something broke")
		if !strings.Contains(html, "Error") {
			t.Error("missing error title")
		}
		if !strings.Contains(html, "something broke") {
			t.Error("missing error message")
		}
		if !strings.Contains(html, "✗") {
			t.Error("missing failure icon")
		}
	})
}

func TestGetUserIDContextKey(t *testing.T) {
	if GetUserIDContextKey() != userIDContextKey {
		t.Error("GetUserIDContextKey should return the package-private constant verbatim")
	}
}
