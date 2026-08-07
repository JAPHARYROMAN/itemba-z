package migrationassurance

import "testing"

func TestValidateReconciliationUsesExactDecimalArithmetic(t *testing.T) {
	file := validReconciliationFile()
	file.Entries[0].ExpectedValue = "10000000000000000.10"
	file.Entries[0].ActualValue = "10000000000000000.100000"
	passed, err := validateReconciliation(file)
	if err != nil || !passed {
		t.Fatalf("expected exact decimal equality, passed=%v err=%v", passed, err)
	}
	file.Entries[0].ActualValue = "10000000000000000.11"
	passed, err = validateReconciliation(file)
	if err != nil || passed {
		t.Fatalf("expected exact decimal mismatch, passed=%v err=%v", passed, err)
	}
}

func TestValidateReconciliationRequiresEveryDomain(t *testing.T) {
	file := validReconciliationFile()
	file.Entries[0].Domain = file.Entries[1].Domain
	if _, err := validateReconciliation(file); err == nil {
		t.Fatal("expected duplicate/missing domain rejection")
	}
}

func validReconciliationFile() ReconciliationFile {
	file := ReconciliationFile{
		SchemaVersion: 1, BatchID: "11111111-1111-4111-8111-111111111111", ApproverReference: "finance-controller",
	}
	for _, domain := range requiredReconciliationDomains {
		file.Entries = append(file.Entries, ReconciliationEntry{
			Domain: domain, ExpectedValue: "100.00", ActualValue: "100.000000", Unit: "MINOR_UNITS",
			EvidenceSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		})
	}
	return file
}
