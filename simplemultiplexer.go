// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"
)

// TODO: refactor Multiplexer type for structured hub to be based off this type.

// SimpleMultiplexer allows multiple subscriptions to be made sharing a single
// message queue from the hub. This means that all the messages for the
// various subscriptions are called back in the order that the messages were
// published. If more than one handler is added to the SimpleMultiplexer that
// matches any given topic, the handlers are called back one after the other
// in the order that they were added.
type SimpleMultiplexer interface {
	Add(topic string, handler func(string, interface{}))
	AddMatch(matcher func(string) bool, handler func(string, interface{}))
	Unsubscribe()
}

type simpleElement struct {
	matcher func(string) bool
	handler func(string, interface{})
}

type simpleMultiplexer struct {
	logger       Logger
	mu           sync.Mutex
	outputs      []simpleElement
	unsubscriber func()
}

// NewMultiplexer creates a new multiplexer for the hub and subscribes it.
// Unsubscribing the multiplexer stops calls for all handlers added.
func (h *SimpleHub) NewMultiplexer() SimpleMultiplexer {
	mp := &simpleMultiplexer{logger: h.logger}
	unsub := h.SubscribeMatch(mp.match, mp.callback)
	mp.unsubscriber = unsub
	return mp
}

// Add a handler for a specific topic.
func (m *simpleMultiplexer) Add(topic string, handler func(string, interface{})) {
	m.AddMatch(equalTopic(topic), handler)
}

// AddMatch adds another handler for any topic that matches the matcher.
func (m *simpleMultiplexer) AddMatch(matcher func(string) bool, handler func(string, interface{})) {
	// This mimics the behaviour of the simple hub itself.
	if handler == nil || matcher == nil {
		// It is safe but useless.
		return
	}
	m.mu.Lock()
	m.outputs = append(m.outputs, simpleElement{matcher: matcher, handler: handler})
	m.mu.Unlock()
}

// Unsubscribe the multiplexer from the simple hub.
func (m *simpleMultiplexer) Unsubscribe() {
	m.mu.Lock()
	unsubscriber := m.unsubscriber
	m.unsubscriber = nil
	m.mu.Unlock()

	if multiUnsubscribeTestHook != nil {
		multiUnsubscribeTestHook()
	}

	if unsubscriber != nil {
		unsubscriber()
	}
}

func (m *simpleMultiplexer) callback(topic string, data interface{}) {
	// Since the callback functions have arbitrary code, don't hold the
	// mutex for the duration of the calls.
	m.mu.Lock()
	outputs := make([]simpleElement, len(m.outputs))
	copy(outputs, m.outputs)
	m.mu.Unlock()

	for _, element := range outputs {
		if element.matcher(topic) {
			element.handler(topic, data)
		}
	}
}

// If any of the topic matchers added for the handlers match the topic, the
// simpleMultiplexer matches.
func (m *simpleMultiplexer) match(topic string) bool {
	// Here we explicitly don't make a copy of the outputs as the match
	// function is going to be called much more often than the callback func.
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, element := range m.outputs {
		if element.matcher(topic) {
			return true
		}
	}
	return false
}
