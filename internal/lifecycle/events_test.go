package lifecycle

import (
	"errors"
	"testing"
)

var errStub = errors.New("stub error")

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
		StackStarting,
		StackStarted,
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

func TestStackEventTypePhaseDetection(t *testing.T) {
	if !StackStarting.IsPre() {
		t.Fatalf("expected StackStarting to be classified as pre-event")
	}
	if StackStarting.IsPost() {
		t.Fatalf("expected StackStarting not to be classified as post-event")
	}
	if !StackStarted.IsPost() {
		t.Fatalf("expected StackStarted to be classified as post-event")
	}
	if StackStarted.IsPre() {
		t.Fatalf("expected StackStarted not to be classified as pre-event")
	}
}

func TestStackEventTypesFireOnBus(t *testing.T) {
	tests := []struct {
		name      string
		eventType EventType
		stackName string
		wantFatal bool
	}{
		{name: "StackStarting is pre — handler error is fatal", eventType: StackStarting, stackName: "web", wantFatal: true},
		{name: "StackStarted is post — handler error is warning", eventType: StackStarted, stackName: "web", wantFatal: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewBus()
			bus.Subscribe(tt.eventType, func(e Event) error {
				if e.StackName != tt.stackName {
					t.Errorf("expected StackName %q, got %q", tt.stackName, e.StackName)
				}
				return errStub
			})

			res := bus.Fire(Event{Type: tt.eventType, StackName: tt.stackName})

			if tt.wantFatal && res.Err == nil {
				t.Fatalf("expected fatal error for pre-event, got none")
			}
			if !tt.wantFatal && res.Err != nil {
				t.Fatalf("expected no fatal error for post-event, got %v", res.Err)
			}
			if !tt.wantFatal && len(res.Warnings) != 1 {
				t.Fatalf("expected one warning for post-event, got %d", len(res.Warnings))
			}
		})
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
