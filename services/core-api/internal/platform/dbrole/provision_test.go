package dbrole

import "testing"

func TestValidateLoginRejectsUnsafeOrWeakConfiguration(t *testing.T) {
	for _, username := range []string{"", "Itemba", "1runtime", "runtime role", "runtime;drop", APIRuntimeGroup, WorkerRuntimeGroup} {
		if ValidateLogin(username, "ci-runtime-password", CapabilityAPI) == nil {
			t.Fatalf("unsafe login accepted: %q", username)
		}
	}
	if ValidateLogin("itembaz_api_runtime", "too-short", CapabilityAPI) == nil {
		t.Fatal("weak password was accepted")
	}
	if err := ValidateLogin("itembaz_api_runtime", "ci-runtime-password", CapabilityAPI); err != nil {
		t.Fatalf("safe login rejected: %v", err)
	}
	if err := ValidateLogin("itembaz_worker_login", "ci-runtime-password", CapabilityWorker); err != nil {
		t.Fatalf("safe worker login rejected: %v", err)
	}
	if ValidateLogin("itembaz_api_runtime", "ci-runtime-password", Capability("unknown")) == nil {
		t.Fatal("unknown runtime capability was accepted")
	}
}
