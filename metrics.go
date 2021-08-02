// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import "time"

// Metrics represents methods for collecting information about the internal
// state of the pubsub.
type Metrics interface {
	// Published metric is used to increment how many published messages are
	// sent per topic.
	Published(topic string)

	// Enqueued metrics increments the number of messages a subscriber has
	// currently. This can be used to see if a subscriber has a backlog of
	// messages pilling up.
	Enqueued(index int)

	// Dequeued metrics decrements the message count once the subscriber has
	// in the pending queue. This doesn't tell you if the message was consumed
	// via the callback, or how long it took.
	Dequeued(index int)

	// Consumed metric identifies the number of consumed messages the subscriber
	// has consumed since a message was enqueued.
	Consumed(index int, duration time.Duration)
}

type noOpMetrics struct{}

func (noOpMetrics) Published(topic string)                     {}
func (noOpMetrics) Enqueued(index int)                         {}
func (noOpMetrics) Dequeued(index int)                         {}
func (noOpMetrics) Consumed(index int, duration time.Duration) {}
