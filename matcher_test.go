// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/pubsub"
)

type MatcherSuite struct {
	testing.IsolationSuite
}

var (
	_ = gc.Suite(&MatcherSuite{})

	first       pubsub.Topic = "first"
	firstdot    pubsub.Topic = "first.next"
	second      pubsub.Topic = "second"
	secondfirst pubsub.Topic = "second.first.next"
	space       pubsub.Topic = "a topic"
)

func (*MatcherSuite) TestTopicMatches(c *gc.C) {
	var matcher pubsub.TopicMatcher = first
	c.Check(matcher.Match(first), jc.IsTrue)
	c.Check(matcher.Match(firstdot), jc.IsFalse)
	c.Check(matcher.Match(second), jc.IsFalse)
	c.Check(matcher.Match(secondfirst), jc.IsFalse)
	c.Check(matcher.Match(space), jc.IsFalse)
}

func (*MatcherSuite) TestMatchAll(c *gc.C) {
	matcher := pubsub.MatchAll
	c.Check(matcher.Match(first), jc.IsTrue)
	c.Check(matcher.Match(firstdot), jc.IsTrue)
	c.Check(matcher.Match(second), jc.IsTrue)
	c.Check(matcher.Match(secondfirst), jc.IsTrue)
	c.Check(matcher.Match(space), jc.IsTrue)
}

func (*MatcherSuite) TestMatchRegexpPanicsOnInvalid(c *gc.C) {
	c.Check(func() { pubsub.MatchRegexp("*") }, gc.PanicMatches, "regexp: Compile.*: error parsing regexp: .*")
}

func (*MatcherSuite) TestMatchRegexp(c *gc.C) {
	matcher := pubsub.MatchRegexp("first.*")
	c.Check(matcher.Match(first), jc.IsTrue)
	c.Check(matcher.Match(firstdot), jc.IsTrue)
	c.Check(matcher.Match(second), jc.IsFalse)
	c.Check(matcher.Match(secondfirst), jc.IsTrue)
	c.Check(matcher.Match(space), jc.IsFalse)
}
