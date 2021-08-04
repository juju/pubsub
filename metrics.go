// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import "time"

// Metrics represents methods for collecting information about the internal
// state of the pubsub.
type Metrics interface {
	// Subscribed metric increments to show the number of subscriptions per
	// hub.
	Subscribed()

	// Unsubscribed metric decrements the number of subscriptions the hub has.
	Unsubscribed()

	// Published metric is used to increment how many published messages are
	// sent per topic.
	Published(topic string)

	// Enqueued metric increments the number of messages a subscriber has
	// currently. This can be used to see if a subscriber has a backlog of
	// messages pilling up.
	Enqueued(ident string)

	// Dequeued metric decrements the message count once the subscriber has
	// in the pending queue. This doesn't tell you if the message was consumed
	// via the callback, or how long it took.
	Dequeued(ident string)

	// Consumed metric increments the number of consumed messages the subscriber
	// has consumed since a message was enqueued.
	Consumed(ident string, duration time.Duration)
}

type noOpMetrics struct{}

func (noOpMetrics) Subscribed()                                   {}
func (noOpMetrics) Unsubscribed()                                 {}
func (noOpMetrics) Published(topic string)                        {}
func (noOpMetrics) Enqueued(ident string)                         {}
func (noOpMetrics) Dequeued(ident string)                         {}
func (noOpMetrics) Consumed(ident string, duration time.Duration) {}
