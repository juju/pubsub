// Copyright 2017 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

// Exported to test matching.
func MultiplexerMatch(m Multiplexer, topic string) bool {
	return m.(*multiplexer).match(topic)
}
