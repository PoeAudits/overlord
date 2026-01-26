package sync_test

import (
	"fmt"

	"github.com/PoeAudits/overlord/internal/sync"
)

// Example demonstrates basic conflict detection usage
func ExampleDetectConflicts() {
	// Note: This example would require a real rsync setup
	// In practice, you would call:
	//
	// conflicts, err := sync.DetectConflicts(
	//     "/local/project/path",
	//     "remote-host",
	//     "/remote/project/path",
	//     []string{"node_modules", ".git"},
	// )
	//
	// if err != nil {
	//     log.Fatal(err)
	// }
	//
	// for _, conflict := range conflicts {
	//     fmt.Printf("%s: %s\n", conflict.Type, conflict.Path)
	// }

	fmt.Println("Conflict detection example")
	// Output: Conflict detection example
}

// Example demonstrates how to interpret conflict types
func ExampleConflictType() {
	conflicts := []sync.Conflict{
		{Path: "local-only.txt", Type: sync.ConflictLocalOnly},
		{Path: "remote-only.md", Type: sync.ConflictRemoteOnly},
		{Path: "both-modified.go", Type: sync.ConflictBothModified},
	}

	for _, c := range conflicts {
		fmt.Printf("%s: %s\n", c.Type.String(), c.Path)
	}

	// Output:
	// Local only: local-only.txt
	// Remote only: remote-only.md
	// Modified on both sides: both-modified.go
}

// Example demonstrates conflict validation
func ExampleConflict_Validate() {
	validConflict := sync.Conflict{
		Path: "file.txt",
		Type: sync.ConflictLocalOnly,
	}

	if err := validConflict.Validate(); err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Valid conflict")
	}

	invalidConflict := sync.Conflict{
		Path: "",
		Type: sync.ConflictLocalOnly,
	}

	if err := invalidConflict.Validate(); err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	// Output:
	// Valid conflict
	// Error: conflict path is required
}
