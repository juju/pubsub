// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"reflect"
	"sync"

	"github.com/juju/errors"
	"github.com/juju/loggo"
	"github.com/juju/utils/deque"
)

var logger = loggo.GetLogger("pubsub.subscriber")

type subscriber struct {
	id int

	topicMatcher TopicMatcher
	handler      func(topic Topic, data interface{})

	mutex   sync.Mutex
	pending *deque.Deque
	closed  chan struct{}
	data    chan struct{}
	done    chan struct{}
}

func newSubscriber(matcher TopicMatcher, handler interface{}) (*subscriber, error) {
	f, err := checkHandler(handler)
	if err != nil {
		return nil, errors.Trace(err)
	}
	logger.Tracef("new subscriber, handler func %v", f)
	// A closed channel is used to provide an immediate route through a select
	// call in the loop function.
	closed := make(chan struct{})
	close(closed)
	sub := &subscriber{
		topicMatcher: matcher,
		handler:      f,
		pending:      deque.New(),
		data:         make(chan struct{}, 1),
		done:         make(chan struct{}),
		closed:       closed,
	}
	go sub.loop()
	logger.Debugf("created subscriber %p for %v", sub, matcher)
	return sub, nil
}

func (s *subscriber) close() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// need to iterate through all the pending calls and make sure the wait group
	// is decremented. this isn't exposed yet, but needs to be.
	for call, ok := s.pending.PopFront(); ok; call, ok = s.pending.PopFront() {
		call.(*handlerCallback).done()
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
		call, empty := s.popOne()
		if empty {
			next = nil
		} else {
			next = s.closed
		}
		// call *should* never be nil as we should only be calling
		// popOne in the situations where there is actually something to pop.
		if call != nil {
			logger.Tracef("exec callback %p (%d) func %p", s, s.id, s.handler)
			s.handler(call.topic, call.data)
			call.done()
		}
	}
}

func (s *subscriber) popOne() (*handlerCallback, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	val, ok := s.pending.PopFront()
	if !ok {
		// nothing to do
		return nil, true
	}
	empty := s.pending.Len() == 0
	return val.(*handlerCallback), empty
}

func (s *subscriber) notify(call *handlerCallback) {
	logger.Tracef("notify %d", s.id)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.pending.PushBack(call)
	if s.pending.Len() == 1 {
		s.data <- struct{}{}
	}
}

// checkHandler makes sure that the handler value passed in is a function
// and has the signature:
//    func(Topic, interface{})
func checkHandler(handler interface{}) (func(Topic, interface{}), error) {
	logger.Tracef("checkHandler, handler func %v", handler)
	if handler == nil {
		return nil, errors.NotValidf("missing handler")
	}
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return nil, errors.NotValidf("handler of type %T", handler)
	}
	var result func(Topic, interface{})
	rt := reflect.TypeOf(result)
	if !t.AssignableTo(rt) {
		return nil, errors.NotValidf("incorrect handler signature")
	}
	f, ok := handler.(func(Topic, interface{}))
	if !ok {
		// This shouldn't happen due to the assignable check just above.
		return nil, errors.NotValidf("incorrect handler signature")
	}
	return f, nil
}
