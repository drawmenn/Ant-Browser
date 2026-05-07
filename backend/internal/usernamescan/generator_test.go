package usernamescan

import "testing"

func TestGenerateCandidatesUsesSourceTerms(t *testing.T) {
	items := GenerateCandidates(GeneratorOptions{
		SourceText:   "James Alex",
		TargetLength: 6,
		MaxResults:   20,
	})
	if len(items) == 0 {
		t.Fatal("expected candidates")
	}
	for _, item := range items {
		if len(item) != 6 {
			t.Fatalf("expected length 6, got %q", item)
		}
	}
}

func TestServiceStartCompletesMockScan(t *testing.T) {
	service := NewService(nil, nil)
	defer service.Stop()
	snapshot := service.Start(StartRequest{
		Candidates: []string{"jamesx", "alexxx"},
		IntervalMs: 100,
	})
	if !snapshot.Running {
		t.Fatal("expected scan to start")
	}
}
