package events

import (
	"sync"

	"github.com/jscyril/golang_music_player/api"
)

// EventBus handles event distribution using channels
type EventBus struct {
	subscribers map[api.EventType][]chan api.AudioEvent
	mu          sync.RWMutex
}

// NewEventBus creates a new event bus
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[api.EventType][]chan api.AudioEvent),
	}
}

// Subscribe returns a channel for receiving events of the specified type
func (b *EventBus) Subscribe(eventType api.EventType) <-chan api.AudioEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan api.AudioEvent, 10)
	b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	return ch
}

// SubscribeAll returns a channel for receiving all event types
func (b *EventBus) SubscribeAll() <-chan api.AudioEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan api.AudioEvent, 20)
	// Subscribe to all known event types
	for _, eventType := range []api.EventType{
		api.EventTrackStarted,
		api.EventTrackEnded,
		api.EventPositionUpdate,
		api.EventError,
		api.EventStateChange,
	} {
		b.subscribers[eventType] = append(b.subscribers[eventType], ch)
	}
	return ch
}

// Publish broadcasts an event to all subscribers of that event type
func (b *EventBus) Publish(event api.AudioEvent) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if subs, ok := b.subscribers[event.Type]; ok {
		for _, ch := range subs {
			select {
			case ch <- event:
			default:
				// Channel full, skip to prevent blocking
			}
		}
	}
}

// Unsubscribe removes a subscriber channel
func (b *EventBus) Unsubscribe(ch <-chan api.AudioEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for eventType, subs := range b.subscribers {
		for i, sub := range subs {
			if sub == ch {
				b.subscribers[eventType] = append(subs[:i], subs[i+1:]...)
				break
			}
		}
	}
}

// Close closes all subscriber channels
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Track closed channels to avoid closing the same channel twice
	closed := make(map[chan api.AudioEvent]bool)

	for _, subs := range b.subscribers {
		for _, ch := range subs {
			if !closed[ch] {
				close(ch)
				closed[ch] = true
			}
		}
	}
	b.subscribers = make(map[api.EventType][]chan api.AudioEvent)
}
