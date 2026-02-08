package cmd

import (
	"strings"
	"testing"
	"time"
)

func TestBackupCommitMessage(t *testing.T) {
	timestamp := time.Date(2026, 2, 7, 14, 30, 0, 0, time.UTC)
	message := backupCommitMessage(timestamp)

	if !strings.Contains(message, "overlord backup: registry + peers at") {
		t.Fatalf("unexpected commit message prefix: %q", message)
	}

	if !strings.Contains(message, timestamp.Format(time.RFC3339)) {
		t.Fatalf("commit message missing RFC3339 timestamp: %q", message)
	}
}
