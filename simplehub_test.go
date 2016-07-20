// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"fmt"
	"sync"
	"time"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type SimpleHubSuite struct {
	testing.LoggingCleanupSuite
}

var (
	_ = gc.Suite(&SimpleHubSuite{})

	veryShortTime = time.Millisecond

	topic pubsub.Topic = "testing"
)

func (*SimpleHubSuite) TestPublishNoSubscribers(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	done := hub.Publish(topic, nil)

	select {
	case <-done:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestPublishOneSubscriber(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		c.Check(topic, gc.Equals, topic)
		c.Check(data, gc.IsNil)
		called = true
	})
	done := hub.Publish(topic, nil)

	select {
	case <-done:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(called, jc.IsTrue)
}

func (*SimpleHubSuite) TestPublishCompleterWaits(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
	})
	done := hub.Publish(topic, nil)

	select {
	case <-done:
		c.Fatal("didn't wait")
	case <-time.After(veryShortTime):
	}

	close(wait)

	select {
	case <-done:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestSubscriberExecsInOrder(c *gc.C) {
	mutex := sync.Mutex{}
	var calls []pubsub.Topic
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe(pubsub.MatchRegexp("test.*"), func(topic pubsub.Topic, data interface{}) {
		mutex.Lock()
		defer mutex.Unlock()
		calls = append(calls, topic)
	})
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(pubsub.Topic(fmt.Sprintf("test.%v", i)), nil)
	}

	select {
	case <-lastCall:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(calls, jc.DeepEquals, []pubsub.Topic{"test.0", "test.1", "test.2", "test.3", "test.4"})
}

func (*SimpleHubSuite) TestPublishNotBlockedByHandlerFunc(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
	})
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(topic, nil)
	}
	// release the handlers
	close(wait)

	select {
	case <-lastCall:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestUnsubcribeWithPendingHandlersMarksDone(c *gc.C) {
	wait := make(chan struct{})
	mutex := sync.Mutex{}
	var calls []pubsub.Topic
	hub := pubsub.NewSimpleHub(nil)
	var unsubscriber pubsub.Unsubscriber
	var err error
	unsubscriber = hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
		mutex.Lock()
		defer mutex.Unlock()
		calls = append(calls, topic)
		unsubscriber.Unsubscribe()
	})
	c.Assert(err, gc.IsNil)
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(topic, nil)
	}
	// release the handlers
	close(wait)

	select {
	case <-lastCall:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	// Only the first handler should execute as we unsubscribe in it.
	c.Assert(calls, jc.DeepEquals, []pubsub.Topic{topic})
}

func (*SimpleHubSuite) TestSubscribeMissingHandler(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	unsubscriber := hub.Subscribe(topic, nil)
	c.Assert(unsubscriber, gc.IsNil)
}

func (*SimpleHubSuite) TestSubscribeMissingMatcher(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	unsubscriber := hub.Subscribe(nil, func(pubsub.Topic, interface{}) {})
	c.Assert(unsubscriber, gc.IsNil)
}

func (*SimpleHubSuite) TestUnsubscribe(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub(nil)
	sub := hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		called = true
	})
	sub.Unsubscribe()
	done := hub.Publish(topic, nil)

	select {
	case <-done:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(called, jc.IsFalse)
}

func (*SimpleHubSuite) TestSubscriberMultipleCallbacks(c *gc.C) {
	firstCalled := false
	secondCalled := false
	thirdCalled := false

	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { firstCalled = true })
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { secondCalled = true })
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { thirdCalled = true })

	done := hub.Publish(topic, nil)

	select {
	case <-done:
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}

	c.Check(firstCalled, jc.IsTrue)
	c.Check(secondCalled, jc.IsTrue)
	c.Check(thirdCalled, jc.IsTrue)
}
