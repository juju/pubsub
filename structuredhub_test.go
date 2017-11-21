// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub_test

import (
	"errors"
	"sync"

	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"
	"gopkg.in/yaml.v2"

	"github.com/juju/pubsub"
)

type StructuredHubSuite struct{}

var _ = gc.Suite(&StructuredHubSuite{})

type Emitter struct {
	Origin  string `json:"origin"`
	Message string `json:"message"`
	ID      int    `json:"id"`
}

type JustOrigin struct {
	Origin string `json:"origin"`
}

type MessageID struct {
	Message string `json:"message"`
	Key     int    `json:"id"`
}

type BadID struct {
	ID string `json:"id"`
}

func (*StructuredHubSuite) TestSubscribeHandler(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	for i, test := range []struct {
		description string
		handler     interface{}
		err         string
	}{
		{
			description: "nil handler",
			err:         "nil handler not valid",
		}, {
			description: "string handler",
			handler:     "a string",
			err:         "handler of type string not valid",
		}, {
			description: "too few args",
			handler:     func(string) {},
			err:         "expected 2 or 3 args, got 1, incorrect handler signature not valid",
		}, {
			description: "too many args",
			handler:     func(string, string, string, string) {},
			err:         "expected 2 or 3 args, got 4, incorrect handler signature not valid",
		}, {
			description: "simple hub handler function",
			handler:     func(string, interface{}) {},
			err:         "second arg should be a structure or map\\[string\\]interface{} for data, incorrect handler signature not valid",
		}, {
			description: "bad return values in handler function",
			handler:     func(string, interface{}, error) error { return nil },
			err:         "expected no return values, got 1, incorrect handler signature not valid",
		}, {
			description: "bad first arg",
			handler:     func(int, map[string]interface{}, error) {},
			err:         "first arg should be a string, incorrect handler signature not valid",
		}, {
			description: "bad second arg",
			handler:     func(string, string, error) {},
			err:         "second arg should be a structure or map\\[string\\]interface{} for data, incorrect handler signature not valid",
		}, {
			description: "bad third arg",
			handler:     func(string, map[string]interface{}, error) {},
			err:         "data type of map\\[string\\]interface{} expects only 2 args, got 3, incorrect handler signature not valid",
		}, {
			description: "accept map[string]interface{}",
			handler:     func(string, map[string]interface{}) {},
		}, {
			description: "bad map[string]string",
			handler:     func(string, map[string]string, error) {},
			err:         "second arg should be a structure or map\\[string\\]interface{} for data, incorrect handler signature not valid",
		}, {
			description: "bad third arg",
			handler:     func(string, Emitter, bool) {},
			err:         "third arg should be error for deserialization errors, incorrect handler signature not valid",
		}, {
			description: "accept struct value",
			handler:     func(string, Emitter, error) {},
		},
	} {
		c.Logf("test %d: %s", i, test.description)
		sub, err := hub.SubscribeMatch(pubsub.MatchAll, test.handler)
		if test.err == "" {
			c.Check(err, jc.ErrorIsNil)
			c.Check(sub, gc.NotNil)
		} else {
			c.Check(err, gc.ErrorMatches, test.err)
			c.Check(sub, gc.IsNil)
		}
	}
}

func (*StructuredHubSuite) TestPublishNil(c *gc.C) {
	called := false
	hub := pubsub.NewStructuredHub(nil)
	unsub, err := hub.Subscribe("topic", func(topic string, data map[string]interface{}) {
		c.Check(data, gc.NotNil)
		c.Check(data, gc.HasLen, 0)
		called = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()

	done, err := hub.Publish("topic", nil)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Check(called, jc.IsTrue)
}

func (*StructuredHubSuite) TestBadPublish(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	done, err := hub.Publish("topic", "hello")
	c.Check(done, gc.IsNil)
	c.Check(err, gc.ErrorMatches, "unmarshalling: json: cannot unmarshal string into Go value of type map\\[string\\]interface {}")
}

func (*StructuredHubSuite) TestPublishDeserialize(c *gc.C) {
	source := Emitter{
		Origin:  "test",
		Message: "hello world",
		ID:      42,
	}
	var (
		originCalled  bool
		messageCalled bool
		mapCalled     bool
	)
	hub := pubsub.NewStructuredHub(nil)
	unsub, err := hub.Subscribe("topic", func(topic string, data JustOrigin, err error) {
		c.Check(err, jc.ErrorIsNil)
		c.Check(topic, gc.Equals, "topic")
		c.Check(data.Origin, gc.Equals, source.Origin)
		originCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	unsub, err = hub.Subscribe("topic", func(topic string, data MessageID, err error) {
		c.Check(err, jc.ErrorIsNil)
		c.Check(topic, gc.Equals, "topic")
		c.Check(data.Message, gc.Equals, source.Message)
		c.Check(data.Key, gc.Equals, source.ID)
		messageCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	unsub, err = hub.Subscribe("topic", func(topic string, data map[string]interface{}) {
		c.Check(topic, gc.Equals, "topic")
		c.Check(data, jc.DeepEquals, map[string]interface{}{
			"origin":  "test",
			"message": "hello world",
			"id":      float64(42), // ints are converted to floats through json.
		})
		mapCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	done, err := hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	// Make sure they were all called.
	c.Check(originCalled, jc.IsTrue)
	c.Check(messageCalled, jc.IsTrue)
	c.Check(mapCalled, jc.IsTrue)
}

func (*StructuredHubSuite) TestPublishMap(c *gc.C) {
	source := map[string]interface{}{
		"origin":  "test",
		"message": "hello world",
		"id":      42,
	}
	var (
		originCalled  bool
		messageCalled bool
		mapCalled     bool
	)
	hub := pubsub.NewStructuredHub(nil)
	unsub, err := hub.Subscribe("topic", func(topic string, data JustOrigin, err error) {
		c.Check(err, jc.ErrorIsNil)
		c.Check(topic, gc.Equals, "topic")
		c.Check(data.Origin, gc.Equals, source["origin"])
		originCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	unsub, err = hub.Subscribe("topic", func(topic string, data MessageID, err error) {
		c.Check(err, jc.ErrorIsNil)
		c.Check(topic, gc.Equals, "topic")
		c.Check(data.Message, gc.Equals, source["message"])
		c.Check(data.Key, gc.Equals, source["id"])
		messageCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	unsub, err = hub.Subscribe("topic", func(topic string, data map[string]interface{}) {
		c.Check(topic, gc.Equals, "topic")
		c.Check(data, jc.DeepEquals, map[string]interface{}{
			"origin":  "test",
			"message": "hello world",
			"id":      42, // published maps don't go through the json map conversino
		})
		mapCalled = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	done, err := hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	// Make sure they were all called.
	c.Check(originCalled, jc.IsTrue)
	c.Check(messageCalled, jc.IsTrue)
	c.Check(mapCalled, jc.IsTrue)
}

func (*StructuredHubSuite) TestPublishDeserializeError(c *gc.C) {
	source := Emitter{
		Origin:  "test",
		Message: "hello world",
		ID:      42,
	}
	called := false
	hub := pubsub.NewStructuredHub(nil)
	unsub, err := hub.Subscribe("topic", func(topic string, data BadID, err error) {
		// In order to support both Go 1.8 and 1.9, we check the common parts
		// of the error.
		// Go 1.9 says:
		//   "unmarshalling data: json: cannot unmarshal number into Go struct field BadID.id of type string"
		// Go 1.8 says:
		//   "unmarshalling data: json: cannot unmarshal number into Go value of type string"
		c.Check(err, gc.ErrorMatches, "unmarshalling data: json: cannot unmarshal number into Go .* of type string")
		c.Check(topic, gc.Equals, "topic")
		c.Check(data.ID, gc.Equals, "")
		called = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	done, err := hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Assert(called, jc.IsTrue)
}

type yamlMarshaller struct{}

func (*yamlMarshaller) Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func (*yamlMarshaller) Unmarshal(data []byte, v interface{}) error {
	return yaml.Unmarshal(data, v)
}

func (*StructuredHubSuite) TestYAMLMarshalling(c *gc.C) {
	source := Emitter{
		Origin:  "test",
		Message: "hello world",
		ID:      42,
	}
	called := false
	hub := pubsub.NewStructuredHub(
		&pubsub.StructuredHubConfig{
			Marshaller: &yamlMarshaller{},
		})
	unsub, err := hub.Subscribe("topic", func(topic string, data map[string]interface{}) {
		c.Check(topic, gc.Equals, "topic")
		c.Check(data, jc.DeepEquals, map[string]interface{}{
			"origin":  "test",
			"message": "hello world",
			"id":      42, // yaml serializes integers just fine.
		})
		called = true
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()

	done, err := hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	// Make sure they were all called.
	c.Assert(called, jc.IsTrue)
}

func (*StructuredHubSuite) TestAnnotations(c *gc.C) {
	source := Emitter{
		Message: "hello world",
		ID:      42,
	}
	origin := "master"
	obtained := []string{}
	hub := pubsub.NewStructuredHub(
		&pubsub.StructuredHubConfig{
			Annotations: map[string]interface{}{
				"origin": origin,
			},
		})
	unsub, err := hub.Subscribe("topic", func(topic string, data Emitter, err error) {
		c.Check(err, jc.ErrorIsNil)
		c.Check(topic, gc.Equals, "topic")
		obtained = append(obtained, data.Origin)
		c.Check(data.Message, gc.Equals, source.Message)
		c.Check(data.ID, gc.Equals, source.ID)
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	done, err := hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	source.Origin = "other"
	done, err = hub.Publish("topic", source)
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Assert(obtained, jc.DeepEquals, []string{origin, "other"})
}

func (*StructuredHubSuite) TestPostProcess(c *gc.C) {
	counter := 0
	values := []int{}
	hub := pubsub.NewStructuredHub(
		&pubsub.StructuredHubConfig{
			PostProcess: func(in map[string]interface{}) (map[string]interface{}, error) {
				counter++
				if counter == 1 {
					return nil, errors.New("bad")
				}
				in["counter"] = counter
				return in, nil
			},
		})
	unsub, err := hub.Subscribe("topic", func(_ string, data map[string]interface{}) {
		values = append(values, data["counter"].(int))
	})
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()

	_, err = hub.Publish("topic", JustOrigin{"origin"})
	c.Assert(err, gc.ErrorMatches, "bad")
	_, err = hub.Publish("topic", JustOrigin{"origin"})
	c.Assert(err, jc.ErrorIsNil)
	done, err := hub.Publish("topic", JustOrigin{"origin"})
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Check(values, jc.DeepEquals, []int{2, 3})
}

type Worker struct {
	m          sync.Mutex
	fromStruct []string
	fromMap    []string
}

func (w *Worker) subMessage(topic string, data MessageID, err error) {
	w.m.Lock()
	defer w.m.Unlock()

	w.fromStruct = append(w.fromStruct, data.Message)
}

func (w *Worker) subData(topic string, data map[string]interface{}) {
	w.m.Lock()
	defer w.m.Unlock()

	value, _ := data["message"].(string)
	w.fromMap = append(w.fromMap, value)
}

func (*StructuredHubSuite) TestMultipleSubscribersSingleInstance(c *gc.C) {
	hub := pubsub.NewStructuredHub(nil)
	w := &Worker{}
	unsub, err := hub.SubscribeMatch(pubsub.MatchAll, w.subData)
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()
	unsub, err = hub.SubscribeMatch(pubsub.MatchAll, w.subMessage)
	c.Assert(err, jc.ErrorIsNil)
	defer unsub()

	message := "a message"
	done, err := hub.Publish("foo", MessageID{Message: message})
	c.Assert(err, jc.ErrorIsNil)

	waitForMessageHandlingToBeComplete(c, done)
	c.Check(w.fromMap, jc.DeepEquals, []string{message})
	c.Check(w.fromStruct, jc.DeepEquals, []string{message})
}
