package lifecycle

import (
	"testing"
)

func TestEventTypeConstantsIncludeAllLifecycleHooks(t *testing.T) {
	events := []EventType{
		ProjectCreating,
		ProjectCreated,
		ProjectOpening,
		ProjectArchiving,
		ProjectArchived,
		ProjectUnarchiving,
		ProjectUnarchived,
		ProjectRemoving,
		ProjectRemoved,
		ProjectInitializing,
	}

	seen := make(map[EventType]struct{})
	for _, event := range events {
		if event == "" {
			t.Fatalf("event type %q must have a non-empty value", event)
		}
		if _, ok := seen[event]; ok {
			t.Fatalf("duplicate event type detected: %s", event)
		}
		seen[event] = struct{}{}
	}

	if len(events) != len(seen) {
		t.Fatalf("expected %d unique lifecycle events, got %d", len(events), len(seen))
	}
}

func TestEventTypePhaseDetection(t *testing.T) {
	tests := []struct {
		name     string
		event    EventType
		wantPre  bool
		wantPost bool
	}{
		{name: "creating", event: ProjectCreating, wantPre: true},
		{name: "opening", event: ProjectOpening, wantPre: true},
		{name: "archiving", event: ProjectArchiving, wantPre: true},
		{name: "unarchiving", event: ProjectUnarchiving, wantPre: true},
		{name: "removing", event: ProjectRemoving, wantPre: true},
		{name: "initializing", event: ProjectInitializing, wantPre: true},
		{name: "created", event: ProjectCreated, wantPost: true},
		{name: "archived", event: ProjectArchived, wantPost: true},
		{name: "unarchived", event: ProjectUnarchived, wantPost: true},
		{name: "removed", event: ProjectRemoved, wantPost: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantPre && !tt.event.IsPre() {
				t.Fatalf("expected %s to be treated as a pre-event", tt.event)
			}
			if tt.wantPost && !tt.event.IsPost() {
				t.Fatalf("expected %s to be treated as a post-event", tt.event)
			}
			if !tt.wantPre && tt.event.IsPre() {
				t.Fatalf("did not expect %s to be classified as pre", tt.event)
			}
			if !tt.wantPost && tt.event.IsPost() {
				t.Fatalf("did not expect %s to be classified as post", tt.event)
			}
		})
	}
}
