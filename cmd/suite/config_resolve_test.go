package main

import (
	"flag"
	"testing"
)

func TestResolveStringPrecedence(t *testing.T) {
	t.Setenv("SUITE_TEST_VALUE", "  from-env  ")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("value", "from-default", "")

	if got := resolveString(fs, "value", "SUITE_TEST_VALUE", "fallback"); got != "from-env" {
		t.Fatalf("environment value = %q, want from-env", got)
	}
	if err := fs.Parse([]string{"--value=from-flag"}); err != nil {
		t.Fatalf("parse flag: %v", err)
	}
	if got := resolveString(fs, "value", "SUITE_TEST_VALUE", "fallback"); got != "from-flag" {
		t.Fatalf("flag value = %q, want from-flag", got)
	}
}

func TestResolveStringFallsBackForEmptyEnvironment(t *testing.T) {
	t.Setenv("SUITE_TEST_EMPTY", "   ")
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("value", "ignored-flag-default", "")
	if got := resolveString(fs, "value", "SUITE_TEST_EMPTY", "fallback"); got != "fallback" {
		t.Fatalf("resolved value = %q, want fallback", got)
	}
}
