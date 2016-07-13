// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"

	"github.com/juju/errors"
)

// Multiplexer allows multiple subscriptions to be made sharing a single
// message queue from the hub. This means that all the messages for the
// various subscriptions are called back in the order that the messages were
// published. If more than on handler is added to the Multiplexer that matches
// any given topic, the handlers are called back one after the other in the
// order that they were added.
type Multiplexer interface {
	TopicMatcher
	Add(matcher TopicMatcher, handler interface{}) error
}

type element struct {
	matcher  TopicMatcher
	callback *structuredCallback
}

type multiplexer struct {
	mu         sync.Mutex
	outputs    []element
	marshaller Marshaller
}

// NewMultiplexer creates a new multiplexer for the hub and subscribes it.
// Unsubscribing the multiplexer stops calls for all handlers added.
// Only structured hubs support multiplexer.
func NewMultiplexer(hub Hub) (Unsubscriber, Multiplexer, error) {
	if hub == nil {
		return nil, nil, errors.NotValidf("nil hub")
	}
	// TODO: make the multiplexer work with a simple hub too.
	// Not needed initially, but it would be nice if it just worked.
	shub, ok := hub.(*structuredHub)
	if !ok {
		return nil, nil, errors.New("hub was not a StructuredHub")
	}
	mp := &multiplexer{marshaller: shub.marshaller}
	unsub, err := hub.Subscribe(mp, mp.callback)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return unsub, mp, nil
}

// Add another topic matcher and handler to the multiplexer.
func (m *multiplexer) Add(matcher TopicMatcher, handler interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	callback, err := newStructuredCallback(m.marshaller, handler)
	if err != nil {
		return errors.Trace(err)
	}
	m.outputs = append(m.outputs, element{matcher: matcher, callback: callback})
	return nil
}

func (m *multiplexer) callback(topic Topic, data map[string]interface{}, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Should never error here.
	if err != nil {
		logger.Errorf("multiplexer callback err: %v", err)
		return
	}
	for _, element := range m.outputs {
		if element.matcher.Match(topic) {
			element.callback.handler(topic, data)
		}
	}
}

// Match implements TopicMatcher. If any of the topic matchers added for the
// handlers match the topic, the multiplexer matches.
func (m *multiplexer) Match(topic Topic) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, element := range m.outputs {
		if element.matcher.Match(topic) {
			return true
		}
	}
	return false
}
