package lifecycle

import (
	"errors"
	"fmt"
)

// Bus manages event subscriptions and dispatch.
type Bus struct {
	handlers map[EventType][]Handler
}

// DefaultBus is the shared bus used in production wiring.
var DefaultBus = NewBus()

// NewBus constructs an event bus.
func NewBus() *Bus {
	return &Bus{handlers: make(map[EventType][]Handler)}
}

// reset clears all registered handlers. For testing only — avoids direct
// field access which bypasses any future synchronization.
func (b *Bus) reset() {
	b.handlers = make(map[EventType][]Handler)
}

// Subscribe registers a handler for an event type.
func (b *Bus) Subscribe(eventType EventType, handler Handler) {
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Fire dispatches an event to subscribed handlers.
func (b *Bus) Fire(event Event) HandlerResult {
	// Copy the handler slice so that any handlers registered during dispatch
	// do not affect the current iteration (future-proofing against mid-fire
	// Subscribe calls mutating the underlying slice).
	handlers := append([]Handler(nil), b.handlers[event.Type]...)
	if len(handlers) == 0 {
		return HandlerResult{}
	}

	result := HandlerResult{}
	var fatalErrors []error

	for i, handler := range handlers {
		err := callHandler(handler, event)
		if err == nil {
			continue
		}
		recordHandlerError(&result, &fatalErrors, event.Type, i, err)
	}

	if len(fatalErrors) > 0 {
		result.Err = errors.Join(fatalErrors...)
	}

	return result
}

func recordHandlerError(result *HandlerResult, fatalErrors *[]error, eventType EventType, handlerIndex int, err error) {
	wrappedErr := fmt.Errorf("handler %d for %s failed: %w", handlerIndex+1, eventType, err)
	if eventType.IsPost() {
		result.Warnings = append(result.Warnings, wrappedErr.Error())
		return
	}
	*fatalErrors = append(*fatalErrors, wrappedErr)
}

func callHandler(handler Handler, event Event) (err error) {
	// Named return allows defer to set err on panic recovery.
	defer func() {
		if recovered := recover(); recovered != nil {
			if rerr, ok := recovered.(error); ok {
				err = fmt.Errorf("handler panic: %w", rerr)
			} else {
				err = fmt.Errorf("handler panic: %v", recovered)
			}
		}
	}()

	return handler(event)
}
