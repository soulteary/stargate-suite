package e2e

import "testing"

// Fixed interoperability vectors for Herald 1.9's HMAC v3 canonical format.
// The two cases distinguish an absent idempotency header from a supplied one.
func TestHeraldV3SigningVector(t *testing.T) {
	cases := []struct {
		key  string
		want string
	}{
		{"", "ae45e7ebae9b3e3bcf5de8b549a1601007fc7b8d8a7b21811905835e1db31e37"},
		{"request-1", "42ac430729d18b640bddf8ff93602d044e75e1c3ae92887c1ab657001fe3be49"},
	}
	for _, tc := range cases {
		got := signHeraldFixed("v3", "POST", "/v1/otp/challenges", "", "1730000000", "nonce-1", "e2e-suite", "default", tc.key, "test-hmac-secret", []byte(`{"purpose":"login"}`))
		if got != tc.want {
			t.Errorf("idempotency key %q: got signature %s, want %s", tc.key, got, tc.want)
		}
	}
}
