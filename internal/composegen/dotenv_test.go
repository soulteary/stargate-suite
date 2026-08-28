package composegen

import (
	"strings"
	"testing"
)

func TestEncodeEnvValue(t *testing.T) {
	tests := map[string]string{
		"":                         "",
		"plain-value_1":            "plain-value_1",
		"$argon2id$v=19$m=65536":   "'$argon2id$v=19$m=65536'",
		"value # not a comment":     "'value # not a comment'",
		"line one\nline two":        "'line one\nline two'",
		"operator's secret":         `'operator\'s secret'`,
		`C:\\path\\with\\slashes`: `'C:\\path\\with\\slashes'`,
	}
	for input, want := range tests {
		if got := EncodeEnvValue(input); got != want {
			t.Errorf("EncodeEnvValue(%q)=%q, want %q", input, got, want)
		}
	}
}

func TestEnvBodyFromVarsQuotesAndSortsRemainingKeys(t *testing.T) {
	body := EnvBodyFromVars(map[string]string{
		"ZZ_EXTRA": "contains $dollar",
		"AA_EXTRA": "value # comment",
	}, "", nil)
	if !strings.Contains(body, "AA_EXTRA='value # comment'\n") {
		t.Fatalf("AA_EXTRA was not safely quoted:\n%s", body)
	}
	if !strings.Contains(body, "ZZ_EXTRA='contains $dollar'\n") {
		t.Fatalf("ZZ_EXTRA was not safely quoted:\n%s", body)
	}
	if strings.Index(body, "AA_EXTRA=") > strings.Index(body, "ZZ_EXTRA=") {
		t.Fatalf("remaining variables are not sorted:\n%s", body)
	}
}

func TestValidEnvKey(t *testing.T) {
	for _, key := range []string{"VALID", "_VALID_2", "lowercase"} {
		if !ValidEnvKey(key) {
			t.Errorf("ValidEnvKey(%q)=false", key)
		}
	}
	for _, key := range []string{"", "2INVALID", "BAD-NAME", "BAD\nINJECT"} {
		if ValidEnvKey(key) {
			t.Errorf("ValidEnvKey(%q)=true", key)
		}
	}
}
