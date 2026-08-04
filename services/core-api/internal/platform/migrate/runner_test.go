package migrate

import "testing"

func TestChecksumIsStableAndContentSensitive(t *testing.T) {
	first := checksum([]byte("SELECT 1;"))
	if first != checksum([]byte("SELECT 1;")) {
		t.Fatal("checksum is not stable")
	}
	if first == checksum([]byte("SELECT 2;")) {
		t.Fatal("checksum ignored content change")
	}
	if len(first) != 64 {
		t.Fatalf("checksum length=%d", len(first))
	}
}
