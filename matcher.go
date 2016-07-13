// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"fmt"
	"regexp"
)

// Match implements TopicMatcher. One topic matches another if they
// are equal.
func (t Topic) Match(topic Topic) bool {
	return t == topic
}

type regexMatcher struct {
	match *regexp.Regexp
}

// MatchRegex expects a valid regular expression. If the expression
// passed in is not valid, the function panics. The expected use of this
// is to be able to do something like:
//
//     hub.Subscribe(pubsub.MatchRegex("prefix.*suffix"), handler)
func MatchRegex(expression string) TopicMatcher {
	matcher, err := regexp.Compile(expression)
	if err != nil {
		panic(fmt.Sprintf("expression must be a valid regular expression: %v", err))
	}
	return &regexMatcher{matcher}
}

// Match implements TopicMatcher. One topic matches another if they
// are equal.
func (m *regexMatcher) Match(topic Topic) bool {
	return m.match.MatchString(string(topic))
}

type allMatcher struct{}

// Match implements TopicMatcher.  All topics match for the allMatcher.
func (*allMatcher) Match(topic Topic) bool {
	return true
}

// MatchAll is a topic matcher that matches all topics.
var MatchAll TopicMatcher = (*allMatcher)(nil)
