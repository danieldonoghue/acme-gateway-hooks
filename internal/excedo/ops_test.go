package excedo

import (
	"testing"
)

func TestZoneCandidates(t *testing.T) {
	got := ZoneCandidates("_acme-challenge.a.b.example.com")
	wantFirst := "a.b.example.com"
	wantLast := "example.com"

	if len(got) < 2 {
		t.Fatalf("expected multiple candidates, got %v", got)
	}
	if got[0] != wantFirst {
		t.Fatalf("first candidate = %q, want %q", got[0], wantFirst)
	}
	if got[len(got)-1] != wantLast {
		t.Fatalf("last candidate = %q, want %q", got[len(got)-1], wantLast)
	}
}

func TestRelativeRecordName(t *testing.T) {
	got := RelativeRecordName("_acme-challenge.api.example.com", "example.com")
	if got != "_acme-challenge.api" {
		t.Fatalf("RelativeRecordName() = %q", got)
	}
}

func TestFindMatchingRecords(t *testing.T) {
	resp := &GetRecordsResponse{
		DNS: map[string]DomainBlock{
			"example.com": {
				Records: []DNSRecord{
					{RecordID: "1", Name: "_acme-challenge", Type: "TXT", Content: "good"},
					{RecordID: "2", Name: "_acme-challenge.example.com", Type: "TXT", Content: "good"},
					{RecordID: "3", Name: "_acme-challenge", Type: "TXT", Content: "other"},
				},
			},
		},
	}

	ids := findMatchingRecords(resp, "_acme-challenge.example.com", "_acme-challenge", "good")
	if len(ids) != 2 {
		t.Fatalf("matched records = %d, want 2", len(ids))
	}
}
