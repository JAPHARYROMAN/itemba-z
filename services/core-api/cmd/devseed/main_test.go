package main

import "testing"

func TestSafeEnvironmentRefusesProductionAndUnset(t *testing.T) {
	for _, value := range []string{"", "production", "PRODUCTION", "staging"} {
		if safeEnvironment(value) {
			t.Fatalf("expected %q to be refused", value)
		}
	}
	for _, value := range []string{"development", "DEV", "local", "test"} {
		if !safeEnvironment(value) {
			t.Fatalf("expected %q to be allowed", value)
		}
	}
}

func TestOfflineFixturePolicyIsPositiveAndCoversSeedProducts(t *testing.T) {
	if offlineTransactionLimitMinor < 8_500_000 || offlineDailyLimitMinor < offlineTransactionLimitMinor {
		t.Fatalf("offline limits do not cover a seeded product sale: transaction=%d daily=%d", offlineTransactionLimitMinor, offlineDailyLimitMinor)
	}
	if offlineProductAllocation <= 0 {
		t.Fatalf("offline product allocation=%d", offlineProductAllocation)
	}
}
