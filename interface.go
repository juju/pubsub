// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

// Topic represents a message that can be subscribed to.
type Topic string

// TopicMatcher defines the Match method that is used to determine
// if the subscriber should be notified about a particular message.
type TopicMatcher interface {
	Match(Topic) bool
}

// Hub represents an in-process delivery mechanism. The hub maintains a
// list of topic subscribers.
type Hub interface {

	// Publish will notifiy all the subscribers that are interested by calling
	// their handler function.
	Publish(topic Topic, data interface{}) (Completer, error)

	// Subscribe takes a topic matcher, and a handler function. If the matcher
	// matches the published topic, the handler function is called. If the
	// handler function does not match what the Hub expects an error is
	// returned. The definition of the handler function depends on the hub
	// implementation. Please see NewSimpleHub and NewStructuredHub.
	Subscribe(matcher TopicMatcher, handler interface{}) (Unsubscriber, error)
}

// Completer provides a way for the caller of publish to know when all of the
// subscribers have finished being notified.
type Completer interface {
	// Complete returns a channel that is closed when all the subscribers
	// have been notified of the event.
	Complete() <-chan struct{}
}

// Unsubscriber provides a simple way to Unsubscribe.
type Unsubscriber interface {
	Unsubscribe()
}
