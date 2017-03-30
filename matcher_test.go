// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type MatcherSuite struct{}

var (
	_ = gc.Suite(&MatcherSuite{})

	first       = "first"
	firstdot    = "first.next"
	second      = "second"
	secondfirst = "second.first.next"
	space       = "a topic"
)

func (*MatcherSuite) TestMatchAll(c *gc.C) {
	matcher := pubsub.MatchAll
	c.Check(matcher(first), jc.IsTrue)
	c.Check(matcher(firstdot), jc.IsTrue)
	c.Check(matcher(second), jc.IsTrue)
	c.Check(matcher(secondfirst), jc.IsTrue)
	c.Check(matcher(space), jc.IsTrue)
}

func (*MatcherSuite) TestMatchRegexpPanicsOnInvalid(c *gc.C) {
	c.Check(func() { pubsub.MatchRegexp("*") }, gc.PanicMatches, "regexp: Compile.*: error parsing regexp: .*")
}

func (*MatcherSuite) TestMatchRegexp(c *gc.C) {
	matcher := pubsub.MatchRegexp("first.*")
	c.Check(matcher(first), jc.IsTrue)
	c.Check(matcher(firstdot), jc.IsTrue)
	c.Check(matcher(second), jc.IsFalse)
	c.Check(matcher(secondfirst), jc.IsTrue)
	c.Check(matcher(space), jc.IsFalse)
}
