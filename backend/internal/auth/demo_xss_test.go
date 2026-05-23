package auth

import (
	"strings"
	"testing"
)

// CF-483 B3: the banner script-tag renderer must escape the demo email
// so a misconfigured DEMO_IDENTITY_EMAIL cannot break out of the JS
// string context and execute arbitrary script. validation.IsValidEmail
// rejects payload-bearing emails at startup, but this is the
// defense-in-depth escaping layer.

func TestRenderDemoBannerScriptTag_EmptyEmail(t *testing.T) {
	if got := RenderDemoBannerScriptTag(""); got != "" {
		t.Errorf("RenderDemoBannerScriptTag(\"\") = %q, want \"\"", got)
	}
}

func TestRenderDemoBannerScriptTag_HappyPath(t *testing.T) {
	out := RenderDemoBannerScriptTag("demo@confabulous.dev")
	if !strings.Contains(out, "window.__DEMO_IDENTITY__") {
		t.Errorf("output missing window.__DEMO_IDENTITY__: %q", out)
	}
	if !strings.Contains(out, "demo@confabulous.dev") {
		t.Errorf("output missing demo email: %q", out)
	}
	if !strings.Contains(out, "<script") || !strings.Contains(out, "</script>") {
		t.Errorf("output not wrapped in <script>...</script>: %q", out)
	}
}

// XSS attempt via embedded </script>. validation.IsValidEmail would
// reject this at boot, but if it ever doesn't, html/template's JS
// escaper must neutralize the breakout.
func TestRenderDemoBannerScriptTag_EscapesScriptCloseTag(t *testing.T) {
	payload := `"</script><script>alert(1)</script>@x`
	out := RenderDemoBannerScriptTag(payload)

	// The literal </script> sequence must not survive in raw form
	// anywhere except the legitimate closing tag of our injected
	// <script>. We check by counting: exactly ONE </script> for the
	// wrapping tag.
	count := strings.Count(out, "</script>")
	if count != 1 {
		t.Errorf("found %d </script> tokens (want exactly 1, our own closer); output:\n%s", count, out)
	}

	// The literal "alert(1)" string is okay to see (it's inside a JS
	// string), but it must not appear adjacent to an unescaped
	// <script> opening tag that would actually execute.
	if strings.Contains(out, "<script>alert(1)") {
		t.Errorf("payload broke out of JS string context; output:\n%s", out)
	}
}

// XSS attempt via raw < character. html/template's JS escape converts
// < to < — anything weaker fails this test.
func TestRenderDemoBannerScriptTag_EscapesAngleBrackets(t *testing.T) {
	out := RenderDemoBannerScriptTag(`<svg/onload=alert(1)>@x`)
	if strings.Contains(out, "<svg/onload") {
		t.Errorf("raw <svg ...> survived escaping; output:\n%s", out)
	}
}

// XSS attempt via raw quote. html/template's JSEscapeString must emit
// an escaped \" rather than a raw " that would otherwise close the
// string literal and start a new statement.
func TestRenderDemoBannerScriptTag_EscapesQuote(t *testing.T) {
	payload := `a"; alert(1); var b="@x`
	out := RenderDemoBannerScriptTag(payload)
	// Strip the opening `= "` and closing `";</script>` framing so we
	// only inspect the JS-string body.
	const prefix = `<script>window.__DEMO_IDENTITY__ = "`
	const suffix = `";</script>`
	if !strings.HasPrefix(out, prefix) || !strings.HasSuffix(out, suffix) {
		t.Fatalf("output not in expected framing; got %q", out)
	}
	body := strings.TrimSuffix(strings.TrimPrefix(out, prefix), suffix)
	// Every " in the body must be preceded by a \ (escaped). An
	// unescaped " would close the string literal.
	for i := 0; i < len(body); i++ {
		if body[i] != '"' {
			continue
		}
		if i == 0 || body[i-1] != '\\' {
			t.Errorf("unescaped \" at offset %d broke out of JS string body=%q", i, body)
		}
	}
}
