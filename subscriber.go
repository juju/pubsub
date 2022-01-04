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
)

type subscriber struct {
	id int

	logger  Logger
	metrics Metrics
	clock   clock.Clock

	topicMatcher func(topic string) bool
	handler      func(topic string, data interface{})
	handlerName  string

	msgQueue *queue
	data     chan struct{}
	done     chan struct{}
}

func newSubscriber(id int,
	matcher func(topic string) bool,
	handler func(string, interface{}),
	logger Logger, metrics Metrics, clock clock.Clock) *subscriber {
	sub := &subscriber{
		id:           id,
		logger:       logger,
		metrics:      metrics,
		clock:        clock,
		topicMatcher: matcher,
		handler:      handler,
		handlerName:  getFunctionName(handler, id),
		msgQueue:     makeQueue(),
		data:         make(chan struct{}, 1),
		done:         make(chan struct{}),
	}
	go sub.loop()
	sub.logger.Tracef("created subscriber %p for %v", sub, matcher)
	return sub
}

func (s *subscriber) close() {
	// need to iterate through all the pending calls and make sure the wait group
	// is decremented. this isn't exposed yet, but needs to be.
	for message, ok := s.msgQueue.Dequeue(); ok; message, ok = s.msgQueue.Dequeue() {
		message.callback.done()

		// Notify the metrics that although we've closed the subscriber, all the
		// messages that subscriber had, have been drained.
		s.metrics.Dequeued(s.handlerName)
	}
	close(s.done)
}

func (s *subscriber) loop() {
	for {
		select {
		case <-s.done:
			return
		case <-s.data:
		}

		for message, ok := s.msgQueue.Dequeue(); ok; message, ok = s.msgQueue.Dequeue() {
			s.metrics.Dequeued(s.handlerName)

			call := message.callback
			s.logger.Tracef("exec callback %p (%d) func %p", s, s.id, s.handler)
			s.handler(call.topic, call.data)
			call.done()

			// Consumed exposes information about how long a given message
			// has been on the subscriber pending list. We can use this
			// information to workout how much backpressure is being exhorted
			// on the system at large.
			s.metrics.Consumed(s.handlerName, s.clock.Now().Sub(message.now))
		}
	}
}

func (s *subscriber) notify(call handlerCallback) {
	s.logger.Tracef("notify %d", s.id)

	s.msgQueue.Enqueue(message{
		now:      s.clock.Now(),
		callback: call,
	})

	select {
	case s.data <- struct{}{}:
	default:
		// Notification pending to be processed
	}

	// Notify the metrics that we're enqueuing a new item onto the subscriber.
	s.metrics.Enqueued(s.handlerName)
}

type message struct {
	callback handlerCallback
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

type queueElement struct {
	msg  message
	next *queueElement
}

type queue struct {
	elemPool sync.Pool

	mu   sync.Mutex
	head *queueElement
	tail *queueElement
}

func makeQueue() *queue {
	return &queue{
		elemPool: sync.Pool{
			New: func() interface{} {
				return new(queueElement)
			},
		},
	}
}

// Enqueue a message onto the queue.
func (q *queue) Enqueue(msg message) {
	elem := q.elemPool.Get().(*queueElement)
	elem.msg = msg

	q.mu.Lock()
	defer q.mu.Unlock()

	if q.head == nil {
		q.head = elem
		q.tail = elem
		return
	}

	q.tail.next = elem
	q.tail = elem
}

// Dequeue a message from the queue. Returns a message or true if it's empty.
func (q *queue) Dequeue() (message, bool) {
	q.mu.Lock()

	if q.head == nil {
		q.mu.Unlock()
		return message{}, false // empty
	}

	elem := q.head
	q.head = elem.next
	if q.head == nil {
		q.tail = nil
	}
	msg := elem.msg
	q.mu.Unlock()

	elem.next = nil
	q.elemPool.Put(elem)
	return msg, true
}
