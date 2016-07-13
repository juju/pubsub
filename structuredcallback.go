// Copyright 2016 Canonical Ltd.
// Licensed under the LGPLv3, see LICENCE file for details.

package pubsub

import (
	"reflect"

	"github.com/juju/errors"
)

type structuredCallback struct {
	marshaller Marshaller
	callback   reflect.Value
	dataType   reflect.Type
}

func newStructuredCallback(marshaller Marshaller, handler interface{}) (*structuredCallback, error) {
	rt, err := checkStructuredHandler(handler)
	if err != nil {
		return nil, errors.Trace(err)
	}
	logger.Tracef("new structured callback, return type %v", rt)
	return &structuredCallback{
		marshaller: marshaller,
		callback:   reflect.ValueOf(handler),
		dataType:   rt,
	}, nil
}

func (s *structuredCallback) handler(topic Topic, data interface{}) {
	var (
		err   error
		value reflect.Value
	)
	asMap, ok := data.(map[string]interface{})
	if !ok {
		err = errors.Errorf("bad data: %v", data)
		value = reflect.Indirect(reflect.New(s.dataType))
	} else {
		logger.Tracef("convert map to %v", s.dataType)
		value, err = toHanderType(s.marshaller, s.dataType, asMap)
	}
	// NOTE: you can't just use reflect.ValueOf(err) as that doesn't work
	// with nil errors. reflect.ValueOf(nil) isn't a valid value. So we need
	// to make  sure that we get the type of the parameter correct, which is
	// the error interface.
	errValue := reflect.Indirect(reflect.ValueOf(&err))
	args := []reflect.Value{reflect.ValueOf(topic), value, errValue}
	s.callback.Call(args)
}

func toHanderType(marshaller Marshaller, rt reflect.Type, data map[string]interface{}) (reflect.Value, error) {
	mapType := reflect.TypeOf(data)
	if mapType == rt {
		return reflect.ValueOf(data), nil
	}
	sv := reflect.New(rt) // returns a Value containing *StructType
	bytes, err := marshaller.Marshal(data)
	if err != nil {
		return reflect.Indirect(sv), errors.Annotate(err, "marshalling data")
	}
	err = marshaller.Unmarshal(bytes, sv.Interface())
	if err != nil {
		return reflect.Indirect(sv), errors.Annotate(err, "unmarshalling data")
	}
	return reflect.Indirect(sv), nil
}

// checkStructuredHandler makes sure that the handler is a function that takes
// a Topic, a structure, and an error. Returns the reflect.Type for the
// structure.
func checkStructuredHandler(handler interface{}) (reflect.Type, error) {
	if handler == nil {
		return nil, errors.NotValidf("nil handler")
	}
	mapType := reflect.TypeOf(map[string]interface{}{})
	t := reflect.TypeOf(handler)
	if t.Kind() != reflect.Func {
		return nil, errors.NotValidf("handler of type %T", handler)
	}
	if t.NumIn() != 3 {
		return nil, errors.NotValidf("expected 3 args, got %d, incorrect handler signature", t.NumIn())
	}
	if t.NumOut() != 0 {
		return nil, errors.NotValidf("expected no return values, got %d, incorrect handler signature", t.NumOut())
	}
	var topic Topic
	var topicType = reflect.TypeOf(topic)

	arg1 := t.In(0)
	arg2 := t.In(1)
	arg3 := t.In(2)
	if arg1 != topicType {
		return nil, errors.NotValidf("first arg should be a pubsub.Topic, incorrect handler signature")
	}
	if arg2.Kind() != reflect.Struct && arg2 != mapType {
		return nil, errors.NotValidf("second arg should be a structure for data, incorrect handler signature")
	}
	if arg3.Kind() != reflect.Interface || arg3.Name() != "error" {
		return nil, errors.NotValidf("third arg should be error for deserialization errors, incorrect handler signature")
	}
	return arg2, nil
}
