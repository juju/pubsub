// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"time"

	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type BenchmarkSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&BenchmarkSuite{})

func (*BenchmarkSuite) BenchmarkStructuredNoConversions(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	topic := pubsub.Topic("benchmarking")
	counter := 0
	sub, err := hub.Subscribe(pubsub.MatchAll, func(topic pubsub.Topic, data map[string]interface{}) {
		counter++
	})
	c.Assert(err, jc.ErrorIsNil)
	defer sub.Unsubscribe()
	failedCount := 0
	for i := 0; i < c.N; i++ {
		done, err := hub.Publish(topic, nil)
		c.Assert(err, jc.ErrorIsNil)

		select {
		case <-done:
		case <-time.After(time.Second):
			failedCount++
		}
	}
	c.Check(failedCount, gc.Equals, 0)
	c.Check(counter, jc.GreaterThan, 0)
}

func (*BenchmarkSuite) BenchmarkStructuredSerialize(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	topic := pubsub.Topic("benchmarking")
	counter := 0
	sub, err := hub.Subscribe(pubsub.MatchAll, func(topic pubsub.Topic, data Emitter, err error) {
		c.Assert(err, jc.ErrorIsNil)
		counter++
	})
	c.Assert(err, jc.ErrorIsNil)
	defer sub.Unsubscribe()
	failedCount := 0
	data := Emitter{
		Origin:  "master",
		Message: "hello world",
		ID:      42,
	}
	for i := 0; i < c.N; i++ {
		done, err := hub.Publish(topic, data)
		c.Assert(err, jc.ErrorIsNil)

		select {
		case <-done:
		case <-time.After(time.Second):
			failedCount++
		}
	}
	c.Check(failedCount, gc.Equals, 0)
	c.Check(counter, jc.GreaterThan, 0)
}
