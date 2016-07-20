// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"

	"github.com/juju/errors"
	"github.com/juju/loggo"
)

// Multiplexer allows multiple subscriptions to be made sharing a single
// message queue from the hub. This means that all the messages for the
// various subscriptions are called back in the order that the messages were
// published. If more than one handler is added to the Multiplexer that
// matches any given topic, the handlers are called back one after the other
// in the order that they were added.
type Multiplexer interface {
	TopicMatcher
	Add(matcher TopicMatcher, handler interface{}) error
}

type element struct {
	matcher  TopicMatcher
	callback *structuredCallback
}

type multiplexer struct {
	logger     loggo.Logger
	mu         sync.Mutex
	outputs    []element
	marshaller Marshaller
}

// NewMultiplexer creates a new multiplexer for the hub and subscribes it.
// Unsubscribing the multiplexer stops calls for all handlers added.
// Only structured hubs support multiplexer.
func (h *StructuredHub) NewMultiplexer() (Unsubscriber, Multiplexer, error) {
	mp := &multiplexer{logger: h.hub.logger, marshaller: h.marshaller}
	unsub, err := h.Subscribe(mp, mp.callback)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return unsub, mp, nil
}

// Add another topic matcher and handler to the multiplexer.
func (m *multiplexer) Add(matcher TopicMatcher, handler interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	callback, err := newStructuredCallback(m.logger, m.marshaller, handler)
	if err != nil {
		return errors.Trace(err)
	}
	m.outputs = append(m.outputs, element{matcher: matcher, callback: callback})
	return nil
}

func (m *multiplexer) callback(topic Topic, data map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
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
