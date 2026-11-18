package bot

import (
	"os"
	"testing"
	"time"
)

func TestJournalAndReporter(t *testing.T) {
	journalPath := "test_journal.log"
	digestPath := "test_digest.json"
	defer os.Remove(journalPath)
	defer os.Remove(digestPath)

	jw, err := NewJournalWriter(journalPath)
	if err != nil {
		t.Fatalf("Failed to create journal writer: %v", err)
	}

	jw.Write("TestEvent", map[string]string{"key": "value"})
	jw.Close()

	// Give time to process
	time.Sleep(100 * time.Millisecond)

	err = GenerateJSONDigest(journalPath, digestPath)
	if err != nil {
		t.Fatalf("Failed to generate digest: %v", err)
	}

	if _, err := os.Stat(digestPath); os.IsNotExist(err) {
		t.Fatalf("Digest file not created")
	}
}
