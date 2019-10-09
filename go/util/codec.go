package util

import (
	"encoding/json"
	"reflect"
)

// Codec serializes/deserializes an instance of a type to/from byte arrays.
// Encode and Decode have to be the inverse of each other.
type Codec interface {
	// Encode serializes the given value to a byte array (inverse of Decode).
	Encode(interface{}) ([]byte, error)

	// Decode deserializes the byte array to an instance of the type that
	// was passed to Encode in a prior call.
	Decode([]byte) (interface{}, error)
}

// JSONCodec implements the Codec interface by serializing and
// deserializing instances of the underlying type of 'instance'.
// Generally it's assumed that 'instance' is a struct, a pointer to
// a struct, a slice or a map.
type JSONCodec struct {
	targetType reflect.Type // the type we want to encode/decode.
	stripPtr   bool         // indicates whether to dereference the pointer of the result.
}

// NewJSONCodec returns a new JSONCodec instance.
func NewJSONCodec(instance interface{}) *JSONCodec {
	targetType := reflect.TypeOf(instance)
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}
	kind := targetType.Kind()
	return &JSONCodec{
		targetType: targetType,
		stripPtr:   (kind == reflect.Slice) || (kind == reflect.Map),
	}
}

// See Codec interface.
func (j *JSONCodec) Encode(data interface{}) ([]byte, error) {
	return json.Marshal(data)
}

// See Codec interface.
func (j *JSONCodec) Decode(byteData []byte) (interface{}, error) {
	// Get a pointer to a new instance, because that's what Unmarshal needs.
	ret := reflect.New(j.targetType).Interface()
	err := json.Unmarshal(byteData, ret)
	if err != nil {
		return nil, err
	} else if j.stripPtr {
		// Strip the pointer for slices and maps.
		return reflect.ValueOf(ret).Elem().Interface(), nil
	}
	return ret, nil
}
