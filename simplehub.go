// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"

	"github.com/juju/clock"
)

var prePublishTestHook func()

// SimpleHubConfig is the argument struct for NewSimpleHub.
type SimpleHubConfig struct {
	// Logger allows specifying a logging implementation for debug
	// and trace level messages emitted from the hub.
	Logger Logger
	// Metrics allows the passing in of a metrics collector.
	Metrics Metrics
	// Clock defines a clock to help improve test coverage.
	Clock clock.Clock
}

// SimpleHub provides the base functionality of dealing with subscribers,
// and the notification of subscribers of events.
type SimpleHub struct {
	mutex       sync.Mutex
	subscribers []*subscriber
	idx         int
	logger      Logger
	metrics     Metrics
	clock       clock.Clock
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
	metrics := config.Metrics
	if metrics == nil {
		metrics = noOpMetrics{}
	}
	time := config.Clock
	if time == nil {
		time = clock.WallClock
	}

	return &SimpleHub{
		logger:  logger,
		metrics: metrics,
		clock:   time,
	}
}

// Publish will notify all the subscribers that are interested by calling
// their handler function.
//
// The data is passed through to each Subscriber untouched. Note that all
// subscribers are notified in parallel, and that no modification should be
// done to the data or data races will occur.
//
// The return function when called blocks and waits for all callbacks to be
// completed.
func (h *SimpleHub) Publish(topic string, data interface{}) func() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	if prePublishTestHook != nil {
		prePublishTestHook()
	}

	var wait sync.WaitGroup
	for _, s := range h.subscribers {
		if s.topicMatcher(topic) {
			wait.Add(1)
			s.notify(&handlerCallback{
				topic: topic,
				data:  data,
				wg:    &wait,
			})
		}
	}

	h.metrics.Published(topic)

	return wait.Wait
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

	sub := newSubscriber(matcher, handler, h.logger, h.metrics, h.clock)
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

// Wait takes the returning function from Publish and returns a channel. The
// channel is closed once the returning function is done.
func Wait(done func()) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		done()
		close(ch)
	}()

	return ch
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
