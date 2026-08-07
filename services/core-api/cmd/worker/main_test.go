package main

import "testing"

func TestLoggingPublisherRequiresExplicitNonProductionEnvironment(t *testing.T) {
	for _, environment := range []string{"", "dev", "Development", "staging", "production", "testing"} {
		if loggingPublisherAllowed(environment) {
			t.Fatalf("environment %q unexpectedly permits the logging publisher", environment)
		}
	}
	for _, environment := range []string{"development", "test"} {
		if !loggingPublisherAllowed(environment) {
			t.Fatalf("environment %q should permit the local logging publisher", environment)
		}
	}
}
