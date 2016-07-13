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
	hub := pubsub.NewSimpleHub()
	result, err := hub.Publish(topic, nil)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case <-result.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestPublishOneSubscriber(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub()
	hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		c.Check(topic, gc.Equals, topic)
		c.Check(data, gc.IsNil)
		called = true
	})
	result, err := hub.Publish(topic, nil)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case <-result.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(called, jc.IsTrue)
}

func (*SimpleHubSuite) TestPublishCompleterWaits(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub()
	hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
	})
	result, err := hub.Publish(topic, nil)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case <-result.Complete():
		c.Fatal("didn't wait")
	case <-time.After(veryShortTime):
	}

	close(wait)

	select {
	case <-result.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestSubscriberExecsInOrder(c *gc.C) {
	mutex := sync.Mutex{}
	var calls []pubsub.Topic
	hub := pubsub.NewSimpleHub()
	_, err := hub.Subscribe(pubsub.MatchRegex("test.*"), func(topic pubsub.Topic, data interface{}) {
		mutex.Lock()
		defer mutex.Unlock()
		calls = append(calls, topic)
	})
	c.Assert(err, gc.IsNil)
	var lastCall pubsub.Completer
	for i := 0; i < 5; i++ {
		lastCall, err = hub.Publish(pubsub.Topic(fmt.Sprintf("test.%v", i)), nil)
		c.Assert(err, jc.ErrorIsNil)
	}

	select {
	case <-lastCall.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(calls, jc.DeepEquals, []pubsub.Topic{"test.0", "test.1", "test.2", "test.3", "test.4"})
}

func (*SimpleHubSuite) TestPublishNotBlockedByHandlerFunc(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub()
	_, err := hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
	})
	c.Assert(err, gc.IsNil)
	var lastCall pubsub.Completer
	for i := 0; i < 5; i++ {
		lastCall, err = hub.Publish(topic, nil)
		c.Assert(err, jc.ErrorIsNil)
	}
	// release the handlers
	close(wait)

	select {
	case <-lastCall.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestUnsubcribeWithPendingHandlersMarksDone(c *gc.C) {
	wait := make(chan struct{})
	mutex := sync.Mutex{}
	var calls []pubsub.Topic
	hub := pubsub.NewSimpleHub()
	var unsubscriber pubsub.Unsubscriber
	var err error
	unsubscriber, err = hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		<-wait
		mutex.Lock()
		defer mutex.Unlock()
		calls = append(calls, topic)
		unsubscriber.Unsubscribe()
	})
	c.Assert(err, gc.IsNil)
	var lastCall pubsub.Completer
	for i := 0; i < 5; i++ {
		lastCall, err = hub.Publish(topic, nil)
		c.Assert(err, jc.ErrorIsNil)
	}
	// release the handlers
	close(wait)

	select {
	case <-lastCall.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	// Only the first handler should execute as we unsubscribe in it.
	c.Assert(calls, jc.DeepEquals, []pubsub.Topic{topic})
}

func (*SimpleHubSuite) TestSubscribeMissingHandler(c *gc.C) {
	hub := pubsub.NewSimpleHub()
	_, err := hub.Subscribe(topic, nil)
	c.Assert(err, gc.ErrorMatches, "missing handler not valid")
}

func (*SimpleHubSuite) TestUnsubscribe(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub()
	sub, err := hub.Subscribe(topic, func(topic pubsub.Topic, data interface{}) {
		called = true
	})
	c.Assert(err, jc.ErrorIsNil)
	sub.Unsubscribe()
	result, err := hub.Publish(topic, nil)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case <-result.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}
	c.Assert(called, jc.IsFalse)
}

func (*SimpleHubSuite) TestSubscriberMultipleCallbacks(c *gc.C) {
	firstCalled := false
	secondCalled := false
	thirdCalled := false

	hub := pubsub.NewSimpleHub()
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { firstCalled = true })
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { secondCalled = true })
	hub.Subscribe(topic, func(pubsub.Topic, interface{}) { thirdCalled = true })

	result, err := hub.Publish(topic, nil)
	c.Assert(err, jc.ErrorIsNil)

	select {
	case <-result.Complete():
	case <-time.After(veryShortTime):
		c.Fatal("publish did not complete")
	}

	c.Check(firstCalled, jc.IsTrue)
	c.Check(secondCalled, jc.IsTrue)
	c.Check(thirdCalled, jc.IsTrue)
}
