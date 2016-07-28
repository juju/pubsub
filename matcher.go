// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import "regexp"

// Topic represents a message that can be subscribed to.
type Topic string

// TopicMatcher defines the Match method that is used to determine
// if the subscriber should be notified about a particular message.
type TopicMatcher interface {
	Match(Topic) bool
}

// Match implements TopicMatcher. One topic matches another if they
// are equal.
func (t Topic) Match(topic Topic) bool {
	return t == topic
}

// RegexpMatcher allows standard regular expressions to be used as
// TopicMatcher values. RegexpMatches can be created using the short-hand
// function MatchRegexp function that wraps regexp.MustCompile.
type RegexpMatcher regexp.Regexp

// MatchRegexp expects a valid regular expression. If the expression
// passed in is not valid, the function panics. The expected use of this
// is to be able to do something like:
//
//     hub.Subscribe(pubsub.MatchRegex("prefix.*suffix"), handler)
func MatchRegexp(expression string) TopicMatcher {
	return (*RegexpMatcher)(regexp.MustCompile(expression))
}

// Match implements TopicMatcher.
//
// The topic matches if the regular expression matches the topic.
func (m *RegexpMatcher) Match(topic Topic) bool {
	r := (*regexp.Regexp)(m)
	return r.MatchString(string(topic))
}

type allMatcher struct{}

// Match implements TopicMatcher.  All topics match for the allMatcher.
func (*allMatcher) Match(topic Topic) bool {
	return true
}

// MatchAll is a topic matcher that matches all topics.
var MatchAll TopicMatcher = (*allMatcher)(nil)
