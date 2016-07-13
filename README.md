
# pubsub
    import "github.com/juju/pubsub"

Package pubsub provides publish and subscribe functionality within a single process.

The core aspect of the pubsub package is the `Hub`. The hub just provides
two methods:
* Publish
* Subscribe

A message as far as a hub is concerned is defined by a topic, and a data
blob. All subscribers that match the published topic are notified, and have
their callback function called with both the topic and the data blob.

All subscribers get their own go routine. This way slow consumers do not
slow down the act of publishing, and slow consumers do not inferfere with
other consumers. Subscribers are guaranteed to get the messages that match
their topic matcher in the order that the messages were published to the
hub.

This package defines two types of Hubs.
* Simple hubs
* Structured hubs

Simple hubs just pass the datablob to the subscribers untouched.
Structuctured hubs will serialize the datablob into a
`map[string]interface{}` using the marshaller that was defined to create
it. The subscription handler functions for structured hubs allow the
handlers to define a structure for the datablob to be marshalled into.

Handler functions for the simple hubs must conform to:


	func (Topic, interface{})

Hander functions for a structured hub can get all the published data available
by defining a callback with the signature:


	func (Topic, map[string]interface{}, error)

Or alternatively, define a struct type, and use that type as the second argument.


	func (Topic, SomeStruct, error)

The structured hub will try to serialize the published information into the
struct specified. If there is an error marshalling, that error is passed to
the callback as the error parameter.





## Variables
``` go
var JSONMarshaller = &jsonMarshaller{}
```
JSONMarshaller simply wraps the json.Marshal and json.Unmarshal calls for the
Marshaller interface.



## type Completer
``` go
type Completer interface {
    // Complete returns a channel that is closed when all the subscribers
    // have been notified of the event.
    Complete() <-chan struct{}
}
```
Completer provides a way for the caller of publish to know when all of the
subscribers have finished being notified.











## type Hub
``` go
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
```
Hub represents an in-process delivery mechanism. The hub maintains a
list of topic subscribers.









### func NewSimpleHub
``` go
func NewSimpleHub() Hub
```
NewSimpleHub returns a new Hub instance.

A simple hub does not touch the data that is passed through to Publish.
This data is passed through to each Subscriber. Note that all subscribers
are notified in parallel, and that no modification should be done to the
data or data races will occur.

All handler functions passed into Subscribe methods of a SimpleHub should
be `func(Topic, interface{})`. The topic of the published method is the first
parameter, and the published data is the seconnd parameter.


### func NewStructuredHub
``` go
func NewStructuredHub(config *StructuredHubConfig) Hub
```
NewStructuredHub returns a new Hub instance.




## type Marshaller
``` go
type Marshaller interface {
    Marshal(interface{}) ([]byte, error)
    Unmarshal([]byte, interface{}) error
}
```
Marshaller defines the Marshal and Unmarshal methods used to serialize and
deserialize the structures used in Publish and Subscription handlers of the
structured hub.











## type Multiplexer
``` go
type Multiplexer interface {
    TopicMatcher
    Add(matcher TopicMatcher, handler interface{}) error
}
```
Multiplexer allows multiple subscriptions to be made sharing a single
message queue from the hub. This means that all the messages for the
various subscriptions are called back in the order that the messages were
published. If more than on handler is added to the Multiplexer that matches
any given topic, the handlers are called back one after the other in the
order that they were added.











## type StructuredHubConfig
``` go
type StructuredHubConfig struct {
    // Marshaller defines how the structured hub will convert from structures to
    // a map[string]interface{} and back. If this is not specified, the
    // `JSONMarshaller` is used.
    Marshaller Marshaller

    // Annotations are added to each message that is published if and only if
    // the values are not already set.
    Annotations map[string]interface{}

    // PostProcess allows the caller to modify the resulting
    // map[string]interface{}.
    PostProcess func(map[string]interface{}) (map[string]interface{}, error)
}
```
StructuredHubConfig is the argument struct for NewStructuredHub.











## type Topic
``` go
type Topic string
```
Topic represents a message that can be subscribed to.











### func (Topic) Match
``` go
func (t Topic) Match(topic Topic) bool
```
Match implements TopicMatcher. One topic matches another if they
are equal.



## type TopicMatcher
``` go
type TopicMatcher interface {
    Match(Topic) bool
}
```
TopicMatcher defines the Match method that is used to determine
if the subscriber should be notified about a particular message.





``` go
var MatchAll TopicMatcher = (*allMatcher)(nil)
```
MatchAll is a topic matcher that matches all topics.





### func MatchRegex
``` go
func MatchRegex(expression string) TopicMatcher
```
MatchRegex expects a valid regular expression. If the expression
passed in is not valid, the function panics. The expected use of this
is to be able to do something like:


	hub.Subscribe(pubsub.MatchRegex("prefix.*suffix"), handler)




## type Unsubscriber
``` go
type Unsubscriber interface {
    Unsubscribe()
}
```
Unsubscriber provides a simple way to Unsubscribe.









### func NewMultiplexer
``` go
func NewMultiplexer(hub Hub) (Unsubscriber, Multiplexer, error)
```
NewMultiplexer creates a new multiplexer for the hub and subscribes it.
Unsubscribing the multiplexer stops calls for all handlers added.
Only structured hubs support multiplexer.










- - -
Generated by [godoc2md](http://godoc.org/github.com/davecheney/godoc2md)