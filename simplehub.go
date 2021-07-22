// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"context"
	"sync"
)

var prePublishTestHook func()

// SimpleHubConfig is the argument struct for NewSimpleHub.
type SimpleHubConfig struct {
	// Logger allows specifying a logging implementation for debug
	// and trace level messages emitted from the hub.
	Logger Logger
}

// NewSimpleHub returns a new SimpleHub instance.
//
// A simple hub does not touch the data that is passed through to Publish.
// This data is passed through to each Subscriber. Note that all subscribers
// are notified in parallel, and that no modification should be done to the
// data or data races will occur.
func NewSimpleHub(config *SimpleHubConfig) *SimpleHub {
	if config == nil {
		config = new(SimpleHubConfig)
	}

	logger := config.Logger
	if logger == nil {
		logger = noOpLogger{}
	}

	return &SimpleHub{
		logger: logger,
	}
}

// SimpleHub provides the base functionality of dealing with subscribers,
// and the notification of subscribers of events.
type SimpleHub struct {
	mutex       sync.Mutex
	subscribers []*subscriber
	idx         int
	logger      Logger
}

// Publish will notifiy all the subscribers that are interested by calling
// their handler function.
//
// The data is passed through to each Subscriber untouched. Note that all
// subscribers are notified in parallel, and that no modification should be
// done to the data or data races will occur.
//
// The channel return value is closed when all the subscribers have been
// notified of the event.
func (h *SimpleHub) Publish(ctx context.Context, topic string, data interface{}) <-chan error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if prePublishTestHook != nil {
		prePublishTestHook()
	}

	var wg sync.WaitGroup

	done := make(chan error)

	for _, s := range h.subscribers {
		if s.topicMatcher(topic) {
			wg.Add(1)
			s.notify(&handlerCallback{
				topic: topic,
				data:  data,
				wg:    &wg,
			})
		}
	}

	go func() {
		// Allow the following waitgroup to unblock after a given set of time.
		// If the notify never finishes, then this go routine will be around
		// for a very long time and never close the done channel; leaking the
		// gorountine.
		if err := waitWithContext(ctx, &wg); err != nil {
			done <- err
			return
		}
		close(done)
	}()

	return done
}

func waitWithContext(ctx context.Context, wg *sync.WaitGroup) error {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ch:
		return nil
	}
}

// Subscribe to a topic with a handler function. If the topic is the same
// as the published topic, the handler function is called with the
// published topic and the associated data.
//
// The return value is a function that will unsubscribe the caller from
// the hub, for this subscription.
func (h *SimpleHub) Subscribe(topic string, handler func(string, interface{})) func() {
	return h.SubscribeMatch(equalTopic(topic), handler)
}

// SubscribeMatch takes a function that determins whether the topic matches,
// and a handler function. If the matcher matches the published topic, the
// handler function is called with the published topic and the associated
// data.
//
// The return value is a function that will unsubscribe the caller from
// the hub, for this subscription.
func (h *SimpleHub) SubscribeMatch(matcher func(string) bool, handler func(string, interface{})) func() {
	if handler == nil || matcher == nil {
		// It is safe but useless.
		return func() {}
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()

	sub := newSubscriber(matcher, handler, h.logger)
	sub.id = h.idx
	h.idx++
	h.subscribers = append(h.subscribers, sub)
	unsub := &handle{hub: h, id: sub.id}
	return unsub.Unsubscribe
}

func (h *SimpleHub) unsubscribe(id int) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	for i, sub := range h.subscribers {
		if sub.id == id {
			sub.close()
			h.subscribers = append(h.subscribers[0:i], h.subscribers[i+1:]...)
			return
		}
	}
}

type handle struct {
	hub *SimpleHub
	id  int
}

// Unsubscribe implements Unsubscriber.
func (h *handle) Unsubscribe() {
	h.hub.unsubscribe(h.id)
}

type handlerCallback struct {
	topic string
	data  interface{}
	wg    *sync.WaitGroup
}

func (h *handlerCallback) done() {
	h.wg.Done()
}
