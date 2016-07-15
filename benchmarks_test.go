// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"time"

	"github.com/juju/pubsub"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

type BenchmarkSuite struct {
	testing.IsolationSuite
}

var _ = gc.Suite(&BenchmarkSuite{})

func (*BenchmarkSuite) BenchmarkStructuredNoConversions(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	topic := pubsub.Topic("benchmarking")
	counter := 0
	sub, err := hub.Subscribe(pubsub.MatchAll, func(topic pubsub.Topic, data map[string]interface{}, err error) {
		counter++
	})
	c.Assert(err, jc.ErrorIsNil)
	defer sub.Unsubscribe()
	failedCount := 0
	for i := 0; i < c.N; i++ {
		result, err := hub.Publish(topic, nil)
		c.Assert(err, jc.ErrorIsNil)

		select {
		case <-result.Complete():
		case <-time.After(5 * veryShortTime):
			failedCount++
		}
	}
}
