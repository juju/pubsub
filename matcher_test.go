// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"github.com/juju/pubsub"
	"github.com/juju/testing"
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
)

type MatcherSuite struct {
	testing.LoggingCleanupSuite
}

var (
	_ = gc.Suite(&MatcherSuite{})

	first    pubsub.Topic = "first"
	firstdot pubsub.Topic = "first.next"
	second   pubsub.Topic = "second"
	space    pubsub.Topic = "a topic"
)

func (*MatcherSuite) TestTopicMatches(c *gc.C) {
	var matcher pubsub.TopicMatcher = first
	c.Assert(matcher.Match(first), jc.IsTrue)
	c.Assert(matcher.Match(firstdot), jc.IsFalse)
	c.Assert(matcher.Match(second), jc.IsFalse)
	c.Assert(matcher.Match(space), jc.IsFalse)
}

func (*MatcherSuite) TestMatchAll(c *gc.C) {
	matcher := pubsub.MatchAll
	c.Assert(matcher.Match(first), jc.IsTrue)
	c.Assert(matcher.Match(firstdot), jc.IsTrue)
	c.Assert(matcher.Match(second), jc.IsTrue)
	c.Assert(matcher.Match(space), jc.IsTrue)
}

func (*MatcherSuite) TestMatchRegexPanicsOnInvalid(c *gc.C) {
	c.Assert(func() { pubsub.MatchRegex("*") }, gc.PanicMatches, "expression must be a valid regular expression: error parsing regexp: .*")
}

func (*MatcherSuite) TestMatchRegex(c *gc.C) {
	matcher := pubsub.MatchRegex("first.*")
	c.Assert(matcher.Match(first), jc.IsTrue)
	c.Assert(matcher.Match(firstdot), jc.IsTrue)
	c.Assert(matcher.Match(second), jc.IsFalse)
	c.Assert(matcher.Match(space), jc.IsFalse)
}
