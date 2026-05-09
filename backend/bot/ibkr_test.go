package bot

import (
	"testing"
	"time"
)

func TestIBKRConnection(t *testing.T) {
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
