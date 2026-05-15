package cmd

import "testing"

func TestTopLevelUpAliasIsRegistered(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"up"})
	if err != nil {
		t.Fatalf("unexpected find error: %v", err)
	}
	if cmd == nil {
		t.Fatal("expected command for 'up'")
	}
	if cmd.Name() != "up" {
		t.Fatalf("expected command name 'up', got %q", cmd.Name())
	}
}
