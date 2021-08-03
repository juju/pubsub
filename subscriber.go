// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/juju/clock"
	"github.com/juju/collections/deque"
)

type subscriber struct {
	id int

	logger  Logger
	metrics Metrics
	clock   clock.Clock

	topicMatcher func(topic string) bool
	handler      func(topic string, data interface{})
	handlerName  string

	mutex   sync.Mutex
	pending *deque.Deque
	closed  chan struct{}
	data    chan struct{}
	done    chan struct{}
}

func newSubscriber(id int,
	matcher func(topic string) bool,
	handler func(string, interface{}),
	logger Logger, metrics Metrics, clock clock.Clock) *subscriber {
	// A closed channel is used to provide an immediate route through a select
	// call in the loop function.
	closed := make(chan struct{})
	close(closed)
	sub := &subscriber{
		id:           id,
		logger:       logger,
		metrics:      metrics,
		clock:        clock,
		topicMatcher: matcher,
		handler:      handler,
		handlerName:  getFunctionName(handler, id),
		pending:      deque.New(),
		data:         make(chan struct{}, 1),
		done:         make(chan struct{}),
		closed:       closed,
	}
	go sub.loop()
	sub.logger.Tracef("created subscriber %p for %v", sub, matcher)
	return sub
}

func (s *subscriber) close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// need to iterate through all the pending calls and make sure the wait group
	// is decremented. this isn't exposed yet, but needs to be.
	for call, ok := s.pending.PopFront(); ok; call, ok = s.pending.PopFront() {
		call.(*message).callback.done()

		// Notify the metrics that although we've closed the subscriber, all the
		// messages that subscriber had, have been drained.
		s.metrics.Dequeued(s.handlerName)
	}
	close(s.done)
}

func (s *subscriber) loop() {
	var next <-chan struct{}
	for {
		select {
		case <-s.done:
			return
		case <-s.data:
			// Has new data been pushed on?
		case <-next:
			// If there was already data, next is a closed channel.
			// otherwise it is nil so won't pass through.
		}
		message, empty := s.popOne()
		if empty {
			next = nil
		} else {
			next = s.closed
		}
		// message *should* never be nil as we should only be calling
		// popOne in the situations where there is actually something to pop.
		if message != nil {
			call := message.callback
			s.logger.Tracef("exec callback %p (%d) func %p", s, s.id, s.handler)
			s.handler(call.topic, call.data)
			call.done()

			// Consumed exposes information about how long a given message
			// has been on the subscriber pending list. We can use this
			// information to workout how much backpressure is being exhorted
			// on the system at large.
			s.metrics.Consumed(s.handlerName, s.clock.Now().Sub(message.now))
		} else {
			// Although it shouldn't happen, we should at least log out when it
			// does, so we can investigate when it does.
			s.logger.Errorf("programatic error: message was nil")
		}
	}
}

func (s *subscriber) popOne() (*message, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	val, ok := s.pending.PopFront()
	if !ok {
		// nothing to do
		return nil, true
	}

	// Notify the metrics that we've dequeued an message from the list.
	s.metrics.Dequeued(s.handlerName)

	empty := s.pending.Len() == 0
	return val.(*message), empty
}

func (s *subscriber) notify(call *handlerCallback) {
	s.logger.Tracef("notify %d", s.id)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.pending.PushBack(&message{
		now:      s.clock.Now(),
		callback: call,
	})
	if s.pending.Len() == 1 {
		s.data <- struct{}{}
	}
	// Notify the metrics that we're enqueuing a new item onto the subscriber.
	s.metrics.Enqueued(s.handlerName)
}

type message struct {
	callback *handlerCallback
	now      time.Time
}

// getFunctionName attempts to return a stable function name that is comparable.
// Restarting the program should return the same function name if the method
// is called on the same function.
func getFunctionName(i interface{}, fallback int) string {
	fullpath := runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
	if len(fullpath) == 0 {
		return fmt.Sprintf("func-%d", fallback)
	}
	parts := strings.Split(fullpath, ".")
	var name string
	switch len(parts) {
	case 0:
		name = fullpath
	case 1:
		name = parts[0]
	default:
		name = strings.Join(parts[len(parts)-2:], ".")
	}
	// Ensure we remove potential suffixes from the name of the function.
	idx := strings.LastIndex(name, "-")
	if idx >= 0 {
		name = name[:idx]
	}
	return name
}
