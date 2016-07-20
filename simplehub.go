// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"

	"github.com/juju/loggo"
)

// SimpleHubConfig is the argument struct for NewSimpleHub.
type SimpleHubConfig struct {
	// LogModule allows for overriding the default logging module.
	// The default value is "pubsub.simple".
	LogModule string
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

	module := "pubsub.simple"
	if config.LogModule != "" {
		module = config.LogModule
	}
	return &SimpleHub{
		logger: loggo.GetLogger(module),
	}
}

// SimpleHub provides the base functionality of dealing with subscribers,
// and the notification of subscribers of events.
type SimpleHub struct {
	mutex       sync.Mutex
	subscribers []*subscriber
	idx         int
	logger      loggo.Logger
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
func (h *SimpleHub) Publish(topic Topic, data interface{}) <-chan struct{} {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	done := make(chan struct{})
	wait := sync.WaitGroup{}

	for _, s := range h.subscribers {
		if s.topicMatcher.Match(topic) {
			wait.Add(1)
			s.notify(
				&handlerCallback{
					topic: topic,
					data:  data,
					wg:    &wait,
				})
		}
	}

	go func() {
		wait.Wait()
		close(done)
	}()

	return done
}

// Subscribe takes a topic matcher, and a handler function. If the matcher
// matches the published topic, the handler function is called with the
// published Topic and the associated data.
//
// The handler function will be called with all maching published events until
// the Unsubscribe method on the Unsubscriber is called.
func (h *SimpleHub) Subscribe(matcher TopicMatcher, handler func(Topic, interface{})) Unsubscriber {
	if handler == nil || matcher == nil {
		return nil
	}
	h.mutex.Lock()
	defer h.mutex.Unlock()

	sub := newSubscriber(matcher, handler, h.logger)
	sub.id = h.idx
	h.idx++
	h.subscribers = append(h.subscribers, sub)
	return &handle{hub: h, id: sub.id}
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
	topic Topic
	data  interface{}
	wg    *sync.WaitGroup
}

func (h *handlerCallback) done() {
	h.wg.Done()
}
