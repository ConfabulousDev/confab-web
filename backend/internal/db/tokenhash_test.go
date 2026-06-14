package db

import "testing"

func TestHashToken(t *testing.T) {
	// Known SHA-256 vector for "abc".
	const abc = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := HashToken("abc"); got != abc {
		t.Errorf("HashToken(\"abc\") = %q, want %q", got, abc)
	}

	// Hex-encoded SHA-256 is always 64 chars (fits web_sessions.id VARCHAR(64)
	// and device_codes.device_code CHAR(64)).
	if got := HashToken("any-high-entropy-token-value"); len(got) != 64 {
		t.Errorf("HashToken length = %d, want 64", len(got))
	}

	// Deterministic, and distinct inputs map to distinct digests.
	a, b := HashToken("token-one"), HashToken("token-two")
	if a == b {
		t.Error("distinct tokens hashed to the same value")
	}
	if a != HashToken("token-one") {
		t.Error("HashToken is not deterministic")
	}

	// The digest never equals the raw input (the whole point — not stored raw).
	if HashToken("raw") == "raw" {
		t.Error("HashToken returned the raw value")
	}
}
