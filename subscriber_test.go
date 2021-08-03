// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub/v2"
)

type SubscriberSuite struct{}

var _ = gc.Suite(&SubscriberSuite{})

func (s SubscriberSuite) TestGetFunctionName(c *gc.C) {
	tests := []struct {
		fn   func()
		name string
	}{{
		fn:   func() {},
		name: "TestGetFunctionName.func1",
	}, {
		fn:   s.method,
		name: "SubscriberSuite.method",
	}, {
		fn:   method,
		name: "com/juju/pubsub/v2_test.method",
	}}

	for _, test := range tests {
		name := pubsub.GetFunctionName(test.fn, 0)
		c.Assert(name, gc.Equals, test.name)
	}
}

func (s SubscriberSuite) method() {}

func method() {}
