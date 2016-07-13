// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"sync"

	"github.com/juju/errors"
	"github.com/juju/loggo"
)

// NewSimpleHub returns a new Hub instance.
//
// A simple hub does not touch the data that is passed through to Publish.
// This data is passed through to each Subscriber. Note that all subscribers
// are notified in parallel, and that no modification should be done to the
// data or data races will occur.
//
// All handler functions passed into Subscribe methods of a SimpleHub should
// be `func(Topic, interface{})`. The topic of the published method is the first
// parameter, and the published data is the seconnd parameter.
func NewSimpleHub() Hub {
	return &simplehub{
		logger: loggo.GetLogger("pubsub.simple"),
	}
}

type simplehub struct {
	mutex       sync.Mutex
	subscribers []*subscriber
	idx         int
	logger      loggo.Logger
}

type doneHandle struct {
	done chan struct{}
}

// Complete implements Completer.
func (d *doneHandle) Complete() <-chan struct{} {
	return d.done
}

// Publish implements Hub.
func (h *simplehub) Publish(topic Topic, data interface{}) (Completer, error) {
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

	return &doneHandle{done: done}, nil
}

// Subscribe implements Hub.
func (h *simplehub) Subscribe(matcher TopicMatcher, handler interface{}) (Unsubscriber, error) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	sub, err := newSubscriber(matcher, handler)
	if err != nil {
		return nil, errors.Trace(err)
	}

	sub.id = h.idx
	h.idx++
	h.subscribers = append(h.subscribers, sub)
	return &handle{hub: h, id: sub.id}, nil
}

func (h *simplehub) unsubscribe(id int) {
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
	hub *simplehub
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
	mu    sync.Mutex
}

func (h *handlerCallback) done() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.wg != nil {
		h.wg.Done()
		h.wg = nil
	}
}
