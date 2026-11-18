package bot

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// JournalEntry represents a single event in the trading journal.
type JournalEntry struct {
	Timestamp time.Time   `json:"timestamp"`
	Type      string      `json:"type"` // e.g., "StateChange", "OrderEvent"
	Data      interface{} `json:"data"`
}

// JournalWriter handles asynchronous logging of events to a file.
type JournalWriter struct {
	mu      sync.Mutex
	file    *os.File
	encoder *json.Encoder
	queue   chan JournalEntry
	done    chan struct{}
}

// NewJournalWriter initializes a new async journal writer.
func NewJournalWriter(filePath string) (*JournalWriter, error) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	jw := &JournalWriter{
		file:    f,
		encoder: json.NewEncoder(f),
		queue:   make(chan JournalEntry, 1000), // Buffered channel for throttling
		done:    make(chan struct{}),
	}

	go jw.process()
	return jw, nil
}

// Write adds an entry to the journal queue.
func (jw *JournalWriter) Write(entryType string, data interface{}) {
	select {
	case jw.queue <- JournalEntry{
		Timestamp: time.Now(),
		Type:      entryType,
		Data:      data,
	}:
	default:
		// Queue full, drop event to avoid blocking engine
		fmt.Fprintf(os.Stderr, "Journal queue full, dropping event: %s\n", entryType)
	}
}

func (jw *JournalWriter) process() {
	for {
		select {
		case entry := <-jw.queue:
			jw.mu.Lock()
			jw.encoder.Encode(entry)
			jw.mu.Unlock()
		case <-jw.done:
			return
		}
	}
}

// Close gracefully shuts down the journal writer.
func (jw *JournalWriter) Close() {
	close(jw.done)
	jw.file.Close()
}
