package lifecycle

import "strings"

// EventType identifies a lifecycle event.
type EventType string

const (
	ProjectCreating     EventType = "project.creating"
	ProjectCreated      EventType = "project.created"
	ProjectOpening      EventType = "project.opening"
	ProjectArchiving    EventType = "project.archiving"
	ProjectArchived     EventType = "project.archived"
	ProjectUnarchiving  EventType = "project.unarchiving"
	ProjectUnarchived   EventType = "project.unarchived"
	ProjectRemoving     EventType = "project.removing"
	ProjectRemoved      EventType = "project.removed"
	ProjectInitializing EventType = "project.initializing"
)

// Event carries context for a lifecycle hook.
type Event struct {
	Type        EventType
	ProjectName string
	ProjectPath string
	DestPath    string
	Settings    map[string]string
}

// Handler defines the callback signature for lifecycle hooks.
type Handler func(Event) error

// HandlerResult aggregates handler execution outcomes.
type HandlerResult struct {
	Warnings []string
	Err      error
}

// IsPre reports whether the event represents a pre-operation phase.
// Detection is based on the string suffix: event types ending in "ing" are
// considered pre-operation hooks. Event types that do not follow this naming
// convention return false. Events without matching counterparts (e.g.,
// ProjectOpening has no ProjectOpened) are valid and still handled correctly.
func (et EventType) IsPre() bool {
	return strings.HasSuffix(string(et), "ing")
}

// IsPost reports whether the event represents a post-operation phase.
// Detection is based on the string suffix: event types ending in "ed" are
// considered post-operation hooks. Event types that do not follow this naming
// convention return false. Events without matching counterparts (e.g.,
// ProjectOpening has no ProjectOpened) are valid and still handled correctly.
func (et EventType) IsPost() bool {
	return strings.HasSuffix(string(et), "ed")
}
