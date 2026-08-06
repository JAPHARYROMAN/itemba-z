package main

import "testing"

func TestLoadCommandRejectsMakerCheckerAndUnsafeEmergencyDuration(t *testing.T) {
	values := map[string]string{
		"ITEMBA_ACCESS_ACTION": "grant-assignment", "ITEMBA_TENANT_ID": "00000000-0000-4000-8000-000000000001",
		"ITEMBA_TARGET_USER_ID": "00000000-0000-4000-8000-000000000002", "ITEMBA_ACTOR_ID": "00000000-0000-4000-8000-000000000003",
		"ITEMBA_APPROVER_ID": "00000000-0000-4000-8000-000000000003", "ITEMBA_ACCESS_REASON": "Verified emergency access",
		"ITEMBA_ACCESS_TICKET": "IAM-123", "ITEMBA_ROLE_ID": "00000000-0000-4000-8000-000000000004",
		"ITEMBA_COMPANY_ID": "00000000-0000-4000-8000-000000000005", "ITEMBA_BRANCH_ID": "00000000-0000-4000-8000-000000000006",
		"ITEMBA_WAREHOUSE_ID": "00000000-0000-4000-8000-000000000007", "ITEMBA_ASSIGNMENT_TYPE": "BREAK_GLASS",
		"ITEMBA_ACCESS_DURATION_SECONDS": "7201",
	}
	for key, value := range values {
		t.Setenv(key, value)
	}
	if _, err := loadCommand(); err == nil {
		t.Fatal("same maker and checker was accepted")
	}
	t.Setenv("ITEMBA_APPROVER_ID", "00000000-0000-4000-8000-000000000008")
	if _, err := loadCommand(); err == nil {
		t.Fatal("break-glass duration above two hours was accepted")
	}
}
