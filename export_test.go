// Copyright 2017 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

var (
	PrePublishTestHook       = &prePublishTestHook
	MultiUnsubscribeTestHook = &multiUnsubscribeTestHook
)

// MultiplexerMatch exported to test matching.
func MultiplexerMatch(m Multiplexer, topic string) bool {
	return m.(*multiplexer).match(topic)
}

// SimpleMultiplexerMatch exported to test matching.
func SimpleMultiplexerMatch(m SimpleMultiplexer, topic string) bool {
	return m.(*simpleMultiplexer).match(topic)
}
