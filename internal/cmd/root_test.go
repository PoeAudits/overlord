package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunRoot_RoutesToListWhenNoArgs(t *testing.T) {
	origRunList := rootRunList
	origRunOpen := rootRunOpen
	t.Cleanup(func() {
		rootRunList = origRunList
		rootRunOpen = origRunOpen
	})

	listCalled := false
	openCalled := false
	rootRunList = func(cmd *cobra.Command, args []string) error {
		listCalled = true
		if len(args) != 0 {
			t.Fatalf("expected no args, got %v", args)
		}
		return nil
	}
	rootRunOpen = func(cmd *cobra.Command, args []string) error {
		openCalled = true
		return nil
	}

	if err := runRoot(&cobra.Command{}, nil); err != nil {
		t.Fatalf("runRoot() returned error: %v", err)
	}

	if !listCalled {
		t.Fatal("expected list route to be called")
	}
	if openCalled {
		t.Fatal("expected open route not to be called")
	}
}

func TestRunRoot_RoutesToOpenWhenArgsPresent(t *testing.T) {
	origRunList := rootRunList
	origRunOpen := rootRunOpen
	t.Cleanup(func() {
		rootRunList = origRunList
		rootRunOpen = origRunOpen
	})

	listCalled := false
	openCalled := false
	rootRunList = func(cmd *cobra.Command, args []string) error {
		listCalled = true
		return nil
	}
	rootRunOpen = func(cmd *cobra.Command, args []string) error {
		openCalled = true
		if len(args) != 1 || args[0] != "dispatch" {
			t.Fatalf("unexpected args: %v", args)
		}
		return nil
	}

	if err := runRoot(&cobra.Command{}, []string{"dispatch"}); err != nil {
		t.Fatalf("runRoot() returned error: %v", err)
	}

	if listCalled {
		t.Fatal("expected list route not to be called")
	}
	if !openCalled {
		t.Fatal("expected open route to be called")
	}
}

func TestFlagErrorFunc_AddUnknownPythonFlagIncludesSuggestionAndUsage(t *testing.T) {
	err := flagErrorFunc(&cobra.Command{Use: "add"}, errors.New("unknown flag: --python"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "Use --py for Python") {
		t.Fatalf("error missing suggestion: %v", err)
	}
	if !strings.Contains(err.Error(), formatAddUsage()) {
		t.Fatalf("error missing add usage: %v", err)
	}
}

func TestFlagErrorFunc_NewRequiredFlagIncludesUsage(t *testing.T) {
	err := flagErrorFunc(&cobra.Command{Use: "new"}, errors.New("required flag(s) \"category\" not set"))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !strings.Contains(err.Error(), "required flag(s) \"category\" not set") {
		t.Fatalf("error missing original message: %v", err)
	}
	if !strings.Contains(err.Error(), formatNewUsage()) {
		t.Fatalf("error missing new usage: %v", err)
	}
}
