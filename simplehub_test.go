// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"fmt"
	"sync"
	"time"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub/v2"
)

type SimpleHubSuite struct{}

var _ = gc.Suite(&SimpleHubSuite{})

func waitForPublishToComplete(c *gc.C, done func()) {
	waitForMessageHandlingToBeComplete(c, pubsub.Wait(done))
}

func waitForMessageHandlingToBeComplete(c *gc.C, done <-chan struct{}) {
	select {
	case <-done:
	case <-time.After(time.Second):
		// We expect message handling to be done in under 1ms
		// so waiting for a second is 1000x as long.
		c.Fatal("publish did not complete")
	}
}

func (*SimpleHubSuite) TestPublishNoSubscribers(c *gc.C) {
	hub := pubsub.NewSimpleHub(nil)
	done := hub.Publish("topic", nil)
	waitForPublishToComplete(c, done)
}

func (*SimpleHubSuite) TestPublishOneSubscriber(c *gc.C) {
	var called bool
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(topic string, data interface{}) {
		c.Check(topic, gc.Equals, "topic")
		c.Check(data, gc.IsNil)
		called = true
	})
	done := hub.Publish("topic", nil)

	waitForPublishToComplete(c, done)
	c.Assert(called, jc.IsTrue)
}

func (*SimpleHubSuite) TestPublishCompleterWaits(c *gc.C) {
	wait := make(chan struct{})
	hub := pubsub.NewSimpleHub(nil)
	hub.Subscribe("topic", func(topic string, data interface{}) {
		<-wait
	})
	done := pubsub.Wait(hub.Publish("topic", nil))

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
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = pubsub.Wait(hub.Publish(fmt.Sprintf("test.%v", i), nil))
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
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = pubsub.Wait(hub.Publish("topic", nil))
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
	var lastCall <-chan struct{}
	for i := 0; i < 5; i++ {
		lastCall = pubsub.Wait(hub.Publish("topic", nil))
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
	done := hub.Publish("topic", nil)

	waitForPublishToComplete(c, done)
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

	done := hub.Publish("topic", nil)

	waitForPublishToComplete(c, done)
	c.Check(firstCalled, jc.IsTrue)
	c.Check(secondCalled, jc.IsTrue)
	c.Check(thirdCalled, jc.IsTrue)
}

func (*SimpleHubSuite) TestMetricsPublished(c *gc.C) {
	metrics := newTestMetrics()

	hub := pubsub.NewSimpleHub(&pubsub.SimpleHubConfig{
		Metrics: metrics,
	})
	done := hub.Publish("topic", nil)
	waitForPublishToComplete(c, done)

	published, queue, consumed := metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{
		"topic": 1,
	})
	c.Assert(queue, gc.DeepEquals, map[string]int{})
	c.Assert(consumed, gc.DeepEquals, map[string]time.Duration{})
}

func (*SimpleHubSuite) TestMetricsQueue(c *gc.C) {
	metrics := newTestMetrics()

	hub := pubsub.NewSimpleHub(&pubsub.SimpleHubConfig{
		Metrics: metrics,
	})
	hub.Subscribe("topic", func(string, interface{}) {})

	published, queue, consumed := metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{})
	c.Assert(queue, gc.DeepEquals, map[string]int{})
	c.Assert(consumed, gc.DeepEquals, map[string]time.Duration{})

	done := hub.Publish("topic", nil)
	waitForPublishToComplete(c, done)

	published, queue, consumed = metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{
		"topic": 1,
	})
	c.Assert(queue, gc.DeepEquals, map[string]int{
		"TestMetricsQueue.func1": 0,
	})
	c.Assert(consumed, gc.HasLen, 1)
}

func (*SimpleHubSuite) TestMetricsDequeue(c *gc.C) {
	metrics := newTestMetrics()

	var (
		first = true

		waiting  sync.WaitGroup
		blocking sync.WaitGroup
	)

	waiting.Add(1)
	blocking.Add(1)

	hub := pubsub.NewSimpleHub(&pubsub.SimpleHubConfig{
		Metrics: metrics,
	})
	hub.Subscribe("topic", func(string, interface{}) {
		if first {
			waiting.Done()
			first = false
		}
		blocking.Wait()
	})

	published, queue, consumed := metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{})
	c.Assert(queue, gc.DeepEquals, map[string]int{})
	c.Assert(consumed, gc.DeepEquals, map[string]time.Duration{})

	hub.Publish("topic", nil)
	done := hub.Publish("topic", nil)

	waiting.Wait()

	published, queue, consumed = metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{
		"topic": 2,
	})
	c.Assert(queue, gc.DeepEquals, map[string]int{
		"TestMetricsDequeue.func1": 1,
	})
	c.Assert(consumed, gc.HasLen, 0)

	blocking.Done()
	waitForPublishToComplete(c, done)

	published, queue, consumed = metrics.Values()
	c.Assert(published, gc.DeepEquals, map[string]int{
		"topic": 2,
	})
	c.Assert(queue, gc.DeepEquals, map[string]int{
		"TestMetricsDequeue.func1": 0,
	})
	c.Assert(consumed, gc.HasLen, 1)
}

type testMetrics struct {
	mutex     sync.Mutex
	published map[string]int
	queue     map[string]int
	consumed  map[string]time.Duration
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		published: make(map[string]int),
		queue:     make(map[string]int),
		consumed:  make(map[string]time.Duration),
	}
}

func (m *testMetrics) Subscribed()   {}
func (m *testMetrics) Unsubscribed() {}

func (m *testMetrics) Published(topic string) {
	m.mutex.Lock()
	m.published[topic]++
	m.mutex.Unlock()
}
func (m *testMetrics) Enqueued(ident string) {
	m.mutex.Lock()
	m.queue[ident]++
	m.mutex.Unlock()
}
func (m *testMetrics) Dequeued(ident string) {
	m.mutex.Lock()
	m.queue[ident]--
	m.mutex.Unlock()
}
func (m *testMetrics) Consumed(ident string, duration time.Duration) {
	m.mutex.Lock()
	m.consumed[ident] = duration
	m.mutex.Unlock()
}

func (m *testMetrics) Values() (map[string]int, map[string]int, map[string]time.Duration) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	p := make(map[string]int, len(m.published))
	for k, v := range m.published {
		p[k] = v
	}
	q := make(map[string]int, len(m.queue))
	for k, v := range m.queue {
		q[k] = v
	}
	c := make(map[string]time.Duration, len(m.consumed))
	for k, v := range m.consumed {
		c[k] = v
	}
	return p, q, c
}
