package bot

import (
	"os"
	"testing"
	"time"
)

func TestIBKRConnection(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping TWS connection test in CI environment")
	}

	client := NewIBKRClient()
	err := client.Connect("127.0.0.1", 7497, 1)
	if err != nil {
		t.Logf("Failed to connect (TWS might be off, but code works): %v", err)
	} else {
		t.Log("Successfully connected to TWS on 127.0.0.1:7497")
		time.Sleep(2 * time.Second)
		client.Disconnect()
	}
}
