// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"time"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type SimpleMultiplexerSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&SimpleMultiplexerSuite{})

func (*SimpleMultiplexerSuite) TestNewMultiplexerStructuredHub(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	multi := hub.NewMultiplexer()
	defer multi.Unsubscribe()
	c.Check(multi, gc.NotNil)
}

func (*SimpleMultiplexerSuite) TestMatcher(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	multi := hub.NewMultiplexer()
	defer multi.Unsubscribe()

	noopFunc := func(string, interface{}) {}
	multi.Add(first, noopFunc)
	multi.AddMatch(pubsub.MatchRegexp("second.*"), noopFunc)

	c.Check(pubsub.SimpleMultiplexerMatch(multi, first), jc.IsTrue)
	c.Check(pubsub.SimpleMultiplexerMatch(multi, firstdot), jc.IsFalse)
	c.Check(pubsub.SimpleMultiplexerMatch(multi, second), jc.IsTrue)
	c.Check(pubsub.SimpleMultiplexerMatch(multi, space), jc.IsFalse)
}

func (*SimpleMultiplexerSuite) TestCallback(c *gc.C) {
	source := &Emission{
		Origin:  "test",
		Message: "hello world",
		ID:      42,
	}
	var (
		topic         = "callback.topic"
		originCalled  bool
		messageCalled bool
	)
	hub := pubsub.NewSimpleHub(nil)
	multi := hub.NewMultiplexer()
	defer multi.Unsubscribe()

	multi.Add(topic, func(top string, data interface{}) {
		c.Check(top, gc.Equals, topic)
		c.Check(data, jc.DeepEquals, source)
		originCalled = true
	})
	multi.Add(second, func(topic string, data interface{}) {
		c.Fail()
		messageCalled = true
	})
	done := hub.Publish(topic, source)

	waitForPublishToComplete(c, done)
	c.Check(originCalled, jc.IsTrue)
	c.Check(messageCalled, jc.IsFalse)
}

func (*SimpleMultiplexerSuite) TestCallbackCanPublish(c *gc.C) {
	var (
		topic         = "callback.topic"
		source        = "magic"
		originCalled  bool
		messageCalled bool
		nestedPublish func()
	)
	hub := pubsub.NewSimpleHub(nil)
	multi := hub.NewMultiplexer()
	defer multi.Unsubscribe()

	multi.Add(topic, func(top string, data interface{}) {
		c.Check(top, gc.Equals, topic)
		c.Check(data, gc.Equals, source)
		originCalled = true

		nestedPublish = hub.Publish(second, "new message")
	})
	multi.Add(second, func(topic string, data interface{}) {
		c.Check(data, gc.Equals, "new message")
		messageCalled = true
	})
	done := hub.Publish(topic, source)

	waitForPublishToComplete(c, done)
	waitForPublishToComplete(c, nestedPublish)
	c.Check(originCalled, jc.IsTrue)
	c.Check(messageCalled, jc.IsTrue)
}

func (s *SimpleMultiplexerSuite) TestUnsubscribeWhileMatch(c *gc.C) {
	// The race we are trying do deal with is this:
	// Publish is called, it acquires underlying mutex on hub
	// Unsubscribe is called, it acquires the mutex on the multiplexer
	// and awaits the mutex on the hub
	// Publish proceeds and calls match on the multiplexer which wants
	// the multiplexer mutex
	// = deadlock

	publishBlock := make(chan struct{})
	publishReady := make(chan struct{})
	unsubBlock := make(chan struct{})
	unsubReady := make(chan struct{})

	s.PatchValue(pubsub.PrePublishTestHook, func() {
		close(publishReady)
		<-publishBlock
	})
	s.PatchValue(pubsub.MultiUnsubscribeTestHook, func() {
		close(unsubReady)
		<-unsubBlock
	})

	hub := pubsub.NewSimpleHub(nil)
	multi := hub.NewMultiplexer()

	multi.Add("a subject", func(string, interface{}) {})

	go func() {
		hub.Publish("test", "a value")
	}()

	select {
	case <-time.After(testing.LongWait):
		c.Errorf("publish not called")
	case <-publishReady:
	}

	unsubscribed := make(chan struct{})
	go func() {
		multi.Unsubscribe()
		close(unsubscribed)
	}()

	select {
	case <-time.After(testing.LongWait):
		c.Errorf("unsubscribe not called")
	case <-unsubReady:
	}

	// Now both functions are in the right place, let them continue.
	close(publishBlock)
	close(unsubBlock)

	select {
	case <-time.After(testing.LongWait):
		c.Errorf("unsubscribe blocked")
	case <-unsubscribed:
	}
}
