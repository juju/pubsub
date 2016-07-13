// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"github.com/juju/errors"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

type SubscriberSuite struct {
	testing.LoggingCleanupSuite
}

var _ = gc.Suite(&SubscriberSuite{})

func (*SubscriberSuite) TestHander(c *gc.C) {
	for i, test := range []struct {
		handler interface{}
		errText string
	}{{
		errText: "missing handler not valid",
	}, {
		handler: "string",
		errText: "handler of type string not valid",
	}, {
		handler: func() {},
		errText: "incorrect handler signature not valid",
	}, {
		handler: func(Topic, TestStruct) {},
		errText: "incorrect handler signature not valid",
	}, {
		handler: func(Topic, interface{}) {},
	}} {
		c.Logf("test %d", i)
		handlerFunc, err := checkHandler(test.handler)
		if test.errText == "" {
			c.Check(err, jc.ErrorIsNil)
			c.Check(handlerFunc, gc.NotNil)
		} else {
			c.Check(err, gc.ErrorMatches, test.errText)
			c.Check(err, jc.Satisfies, errors.IsNotValid)
			c.Check(handlerFunc, gc.IsNil)
		}
	}
}

type TestStruct struct {
	Message string
}
