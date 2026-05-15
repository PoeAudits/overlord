package lifecycle

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSubscribeRegistersHandlersPerEvent(t *testing.T) {
	bus := NewBus()
	handler := func(Event) error { return nil }

	bus.Subscribe(ProjectCreating, handler)
	bus.Subscribe(ProjectCreating, handler)
	bus.Subscribe(ProjectArchived, handler)

	if got := len(bus.handlers[ProjectCreating]); got != 2 {
		t.Fatalf("expected 2 handlers for ProjectCreating, got %d", got)
	}
	if got := len(bus.handlers[ProjectArchived]); got != 1 {
		t.Fatalf("expected 1 handler for ProjectArchived, got %d", got)
	}
}

func TestFireDispatchesHandlersInOrder(t *testing.T) {
	bus := NewBus()
	var calls []string
	mkHandler := func(label string) Handler {
		return func(Event) error {
			calls = append(calls, label)
			return nil
		}
	}

	bus.Subscribe(ProjectCreated, mkHandler("first"))
	bus.Subscribe(ProjectCreated, mkHandler("second"))
	res := bus.Fire(Event{Type: ProjectCreated})

	if res.Err != nil {
		t.Fatalf("expected no error, got %v", res.Err)
	}
	if len(calls) != 2 {
		t.Fatalf("expected both handlers to run, got %d calls", len(calls))
	}
	if calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("handlers ran out of order: %v", calls)
	}
}

func TestFirePreEventErrorsAreFatal(t *testing.T) {
	bus := NewBus()
	secondRan := false
	bus.Subscribe(ProjectCreating, func(Event) error {
		return errors.New("permission denied")
	})
	bus.Subscribe(ProjectCreating, func(Event) error {
		secondRan = true
		return nil
	})

	res := bus.Fire(Event{Type: ProjectCreating})

	if res.Err == nil || !strings.Contains(res.Err.Error(), "permission denied") {
		t.Fatalf("expected fatal error to be returned: %+v", res.Err)
	}
	if !secondRan {
		t.Fatalf("expected subsequent handlers to still execute in order")
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("pre-event errors should not be downgraded to warnings: %+v", res.Warnings)
	}
}

func TestFireJoinsMultiplePreEventErrors(t *testing.T) {
	bus := NewBus()
	first := errors.New("first failure")
	second := errors.New("second failure")

	bus.Subscribe(ProjectCreating, func(Event) error { return first })
	bus.Subscribe(ProjectCreating, func(Event) error { return second })

	res := bus.Fire(Event{Type: ProjectCreating})

	if res.Err == nil {
		t.Fatal("expected joined fatal error, got nil")
	}
	if !errors.Is(res.Err, first) {
		t.Fatalf("expected joined error to contain first failure: %v", res.Err)
	}
	if !errors.Is(res.Err, second) {
		t.Fatalf("expected joined error to contain second failure: %v", res.Err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("pre-event warnings should be empty: %+v", res.Warnings)
	}
}

func TestFirePostEventErrorsBecomeWarnings(t *testing.T) {
	bus := NewBus()
	bus.Subscribe(ProjectArchived, func(Event) error {
		return errors.New("cleanup failed")
	})

	res := bus.Fire(Event{Type: ProjectArchived})

	if res.Err != nil {
		t.Fatalf("post-event errors should not be fatal: %v", res.Err)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected warning to be recorded, got %d", len(res.Warnings))
	}
	if !strings.Contains(res.Warnings[0], "cleanup failed") {
		t.Fatalf("warning should include handler error message: %s", res.Warnings[0])
	}
}

func TestFireCollectsAllPostEventWarnings(t *testing.T) {
	bus := NewBus()
	bus.Subscribe(ProjectArchived, func(Event) error { return errors.New("cleanup failed") })
	bus.Subscribe(ProjectArchived, func(Event) error { return errors.New("metrics failed") })

	res := bus.Fire(Event{Type: ProjectArchived})

	if res.Err != nil {
		t.Fatalf("post-event errors should not be fatal: %v", res.Err)
	}
	if len(res.Warnings) != 2 {
		t.Fatalf("expected 2 warnings, got %d", len(res.Warnings))
	}
	if !strings.Contains(res.Warnings[0], "cleanup failed") {
		t.Fatalf("first warning missing handler error: %s", res.Warnings[0])
	}
	if !strings.Contains(res.Warnings[1], "metrics failed") {
		t.Fatalf("second warning missing handler error: %s", res.Warnings[1])
	}
}

func TestFireRecoversHandlerPanics(t *testing.T) {
	tests := []struct {
		name     string
		event    EventType
		wantErr  bool
		wantWarn bool
	}{
		{name: "pre event", event: ProjectRemoving, wantErr: true},
		{name: "post event", event: ProjectRemoved, wantWarn: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bus := NewBus()
			bus.Subscribe(tt.event, func(Event) error {
				panic("unexpected handler panic")
			})

			res := bus.Fire(Event{Type: tt.event})

			if tt.wantErr && res.Err == nil {
				t.Fatalf("expected panic to be surfaced as error")
			}
			if !tt.wantErr && res.Err != nil {
				t.Fatalf("did not expect fatal error for post-event panic: %v", res.Err)
			}
			if tt.wantWarn {
				if len(res.Warnings) != 1 {
					t.Fatalf("expected warning for post-event panic, got %d", len(res.Warnings))
				}
				if !strings.Contains(res.Warnings[0], "panic") {
					t.Fatalf("warning should mention panic: %s", res.Warnings[0])
				}
			}
		})
	}
}

func TestFirePanicWithErrorPreservesChain(t *testing.T) {
	sentinel := errors.New("underlying cause")

	bus := NewBus()
	bus.Subscribe(ProjectRemoving, func(Event) error {
		panic(sentinel)
	})

	res := bus.Fire(Event{Type: ProjectRemoving})

	if res.Err == nil {
		t.Fatalf("expected error from panicking handler")
	}
	if !errors.Is(res.Err, sentinel) {
		t.Fatalf("expected error chain to contain sentinel via errors.Is; got: %v", res.Err)
	}
}

func TestFirePanicWithNonErrorUsesFormatV(t *testing.T) {
	bus := NewBus()
	bus.Subscribe(ProjectRemoving, func(Event) error {
		panic("string panic value")
	})

	res := bus.Fire(Event{Type: ProjectRemoving})

	if res.Err == nil {
		t.Fatalf("expected error from panicking handler")
	}
	if !strings.Contains(res.Err.Error(), "string panic value") {
		t.Fatalf("expected error to contain panic string; got: %v", res.Err)
	}
}

func TestFireCopiesHandlersBeforeIteration(t *testing.T) {
	bus := NewBus()
	callCount := 0
	bus.Subscribe(ProjectCreated, func(Event) error {
		callCount++
		bus.Subscribe(ProjectCreated, func(Event) error {
			callCount++
			return nil
		})
		return nil
	})

	res := bus.Fire(Event{Type: ProjectCreated})
	if res.Err != nil {
		t.Fatalf("expected no error, got %v", res.Err)
	}
	if callCount != 1 {
		t.Fatalf("expected new handler not to run during same dispatch, callCount=%d", callCount)
	}

	bus.Fire(Event{Type: ProjectCreated})
	if callCount != 3 {
		t.Fatalf("expected both handlers to run during subsequent dispatch, callCount=%d", callCount)
	}
}

func TestDefaultBusIsReadyForRegistration(t *testing.T) {
	if DefaultBus == nil {
		t.Fatalf("DefaultBus should be initialized for Register pattern")
	}

	var executed bool
	DefaultBus.Subscribe(ProjectInitializing, func(Event) error {
		executed = true
		return nil
	})

	DefaultBus.Fire(Event{Type: ProjectInitializing})
	if !executed {
		t.Fatalf("expected handlers registered via Register pattern to run")
	}

	// reset handlers to avoid impacting other tests
	DefaultBus.reset()
}

func BenchmarkBusFire(b *testing.B) {
	bus := NewBus()
	bus.Subscribe(ProjectOpening, func(Event) error { return nil })
	bus.Subscribe(ProjectOpening, func(Event) error { return fmt.Errorf("warning %d", 42) })

	event := Event{Type: ProjectOpening, ProjectName: "bench"}
	for i := 0; i < b.N; i++ {
		bus.Fire(event)
	}
}
