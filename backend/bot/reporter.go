package bot

import (
	"encoding/json"
	"os"
	"time"
)

// Digest represents a summary report.
type Digest struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Entries     []JournalEntry `json:"entries"`
}

// GenerateJSONDigest reads the journal and generates a JSON digest.
func GenerateJSONDigest(journalPath string, digestPath string) error {
	f, err := os.Open(journalPath)
	if err != nil {
		return err
	}
	defer f.Close()

	decoder := json.NewDecoder(f)
	var entries []JournalEntry
	for decoder.More() {
		var entry JournalEntry
		if err := decoder.Decode(&entry); err != nil {
			continue // Skip malformed entries
		}
		entries = append(entries, entry)
	}

	digest := Digest{
		GeneratedAt: time.Now(),
		Entries:     entries,
	}

	data, err := json.MarshalIndent(digest, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(digestPath, data, 0644)
}
