package main

import "testing"

func TestUnsafeFallbackRequiresExactDevelopmentEnvironment(t *testing.T) {
	for _, environment := range []string{"", "dev", "Development", "staging", "production", "developmnt"} {
		if allowsUnsafeFallback(environment) {
			t.Fatalf("environment %q unexpectedly permits unsafe fallback", environment)
		}
	}
	if !allowsUnsafeFallback("development") {
		t.Fatal("explicit development must permit local fallback")
	}
}
