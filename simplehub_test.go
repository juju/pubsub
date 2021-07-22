// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type SimpleHubSuite struct{}

var _ = gc.Suite(&SimpleHubSuite{})

func waitForMessageHandlingToBeComplete(c *gc.C, done <-chan error) {
	select {
	case err := <-done:
		c.Assert(err, jc.ErrorIsNil)
	case <-time.After(time.Second):
		// We expect message handling to be done in under 1ms
		// so waiting for a second is 1000x as long.
		c.Fatal("publish did not complete")
	}
}

func waitForMessageHandlingToBeCancelled(c *gc.C, done <-chan error) {
	select {
	case err := <-done:
		c.Assert(err, gc.ErrorMatches, `context deadline exceeded`)
	case <-time.After(time.Second):
		// We expect message handling to be done in under 1ms
		// so waiting for a second is 1000x as long.
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestPublishNoSubscribers(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	done := hub.Publish(context.TODO(), "topic", nil)
	waitForMessageHandlingToBeComplete(c, done)
}

func (*SimpleHubSuite) TestPublishOneSubscriber(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(topic string, data interface{}) {
		c.Check(topic, gc.Equals, "topic")
		c.Check(data, gc.IsNil)
		called = true
	})
	done := hub.Publish(context.TODO(), "topic", nil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Assert(called, jc.IsTrue)
}

func (*SimpleHubSuite) TestPublishCompleterWaits(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(topic string, data interface{}) {
		<-wait
	})
	done := hub.Publish(context.TODO(), "topic", nil)

	select {
	case <-done:
		c.Fatal("didn't wait")
	case <-time.After(time.Millisecond):
		// Under normal circumstances, this would take ~3000ns
		// See benchmark tests for speed.
	}

	close(wait)
	waitForMessageHandlingToBeComplete(c, done)
}

func (*SimpleHubSuite) TestSubscriberExecsInOrder(c *gc.C) {
	var calls []string
	hub := pubsub.NewSimpleHub(nil)
	hub.SubscribeMatch(pubsub.MatchRegexp("test.*"), func(topic string, data interface{}) {
		calls = append(calls, topic)
	})
	var lastCall <-chan error
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(context.TODO(), fmt.Sprintf("test.%v", i), nil)
	}

	waitForMessageHandlingToBeComplete(c, lastCall)
	c.Assert(calls, jc.DeepEquals, []string{"test.0", "test.1", "test.2", "test.3", "test.4"})
}

func (*SimpleHubSuite) TestPublishNotBlockedByHandlerFunc(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(topic string, data interface{}) {
		<-wait
	})
	var lastCall <-chan error
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(context.TODO(), "topic", nil)
	}
	// release the handlers
	close(wait)

	waitForMessageHandlingToBeComplete(c, lastCall)
}

func (*SimpleHubSuite) TestUnsubcribeWithPendingHandlersMarksDone(c *gc.C) {
	wait := make(chan struct{})
	var calls []string
	hub := pubsub.NewSimpleHub(nil)
	var unsubscribe func()
	var err error
	unsubscribe = hub.Subscribe("topic", func(topic string, data interface{}) {
		<-wait
		calls = append(calls, topic)
		unsubscribe()
	})
	c.Assert(err, gc.IsNil)
	var lastCall <-chan error
	for i := 0; i < 5; i++ {
		lastCall = hub.Publish(context.TODO(), "topic", nil)
	}
	// release the handlers
	close(wait)

	waitForMessageHandlingToBeComplete(c, lastCall)
	// Only the first handler should execute as we unsubscribe in it.
	c.Assert(calls, jc.DeepEquals, []string{"topic"})
}

func (*SimpleHubSuite) TestSubscribeMissingHandler(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	unsubscribe := hub.Subscribe("topic", nil)
	c.Assert(unsubscribe, gc.NotNil)
	// Calling it is fine.
	unsubscribe()
}

func (*SimpleHubSuite) TestSubscribeMissingMatcher(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	unsubscribe := hub.SubscribeMatch(nil, func(string, interface{}) {})
	c.Assert(unsubscribe, gc.NotNil)
	// Calling it is fine.
	unsubscribe()
}

func (*SimpleHubSuite) TestUnsubscribe(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub(nil)
	unsub := hub.Subscribe("topic", func(topic string, data interface{}) {
		called = true
	})
	unsub()
	done := hub.Publish(context.TODO(), "topic", nil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Assert(called, jc.IsFalse)
}

func (*SimpleHubSuite) TestSubscriberMultipleCallbacks(c *gc.C) {
	firstCalled := false
	secondCalled := false
	thirdCalled := false

	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(string, interface{}) { firstCalled = true })
	hub.Subscribe("topic", func(string, interface{}) { secondCalled = true })
	hub.Subscribe("topic", func(string, interface{}) { thirdCalled = true })

	done := hub.Publish(context.TODO(), "topic", nil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Check(firstCalled, jc.IsTrue)
	c.Check(secondCalled, jc.IsTrue)
	c.Check(thirdCalled, jc.IsTrue)
}

func (*SimpleHubSuite) TestPublishDoesNotBlock(c *gc.C) {
	// mutex required because of publish in a goroutine
	var m sync.Mutex
	firstCalled := false
	secondCalled := false

	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithTimeout(context.TODO(), time.Millisecond*500)
	defer cancel()

	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic1", func(string, interface{}) {
		m.Lock()
		firstCalled = true
		m.Unlock()
		// This replicates a slow handler (think API request). There is no
		// way to monitor done here and so we leak a goroutine as we never wait
		// to complete.
		wg.Wait()
	})
	hub.Subscribe("topic2", func(string, interface{}) {
		m.Lock()
		secondCalled = true
		m.Unlock()
	})

	var wg2 sync.WaitGroup
	wg2.Add(2)
	go func() {
		done := hub.Publish(ctx, "topic1", nil)
		waitForMessageHandlingToBeCancelled(c, done)
		wg2.Done()
	}()
	go func() {
		done := hub.Publish(ctx, "topic2", nil)
		waitForMessageHandlingToBeComplete(c, done)
		wg2.Done()
	}()

	wg2.Wait()
	wg.Done()

	m.Lock()
	c.Check(firstCalled, jc.IsTrue)
	c.Check(secondCalled, jc.IsTrue)
	m.Unlock()
}
