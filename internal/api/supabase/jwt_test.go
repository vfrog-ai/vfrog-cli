package supabase

import (
	"testing"
)

func TestDecodeJWT(t *testing.T) {
	// This is a minimal test - in production you'd want to test with real JWTs
	// For now, just test that invalid tokens are rejected
	_, err := DecodeJWT("invalid.token.here")
	if err == nil {
		t.Error("DecodeJWT should fail on invalid token")
	}

	_, err = DecodeJWT("not-enough-parts")
	if err == nil {
		t.Error("DecodeJWT should fail on token with wrong number of parts")
	}
}

