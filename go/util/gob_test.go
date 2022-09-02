package util

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
)

type Item struct {
	Id    string
	Map   map[string]string
	Slice []string
}

func TestGobEncoder(t *testing.T) {
	// TODO(benjaminwagner): Is there any way to cause an error?
	e := GobEncoder{}
	expectedItems := map[*Item][]byte{}
	for i := 0; i < 25; i++ {
		item := &Item{}
		item.Id = fmt.Sprintf("Id-%d", i)
		item.Map = map[string]string{"PointA": "PointB"}
		item.Slice = []string{"bread"}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(item)
		assert.NoError(t, err)
		expectedItems[item] = buf.Bytes()
		assert.True(t, e.Process(item))
	}

	actualItems := map[*Item][]byte{}
	for item, serialized, err := e.Next(); item != nil; item, serialized, err = e.Next() {
		assert.NoError(t, err)
		actualItems[item.(*Item)] = serialized
	}

	assertdeep.Equal(t, expectedItems, actualItems)
}

func TestGobEncoderNoItems(t *testing.T) {
	e := GobEncoder{}
	item, serialized, err := e.Next()
	assert.NoError(t, err)
	assert.Nil(t, item)
	assert.Nil(t, serialized)
}

func TestGobDecoder(t *testing.T) {
	d := NewGobDecoder(func() interface{} {
		return &Item{}
	}, func(ch <-chan interface{}) interface{} {
		items := []*Item{}
		for item := range ch {
			items = append(items, item.(*Item))
		}
		return items
	})
	expectedItems := map[string]*Item{}
	for i := 0; i < 250; i++ {
		item := &Item{}
		item.Id = fmt.Sprintf("Id-%d", i)
		item.Map = map[string]string{"PointA": "PointB"}
		item.Slice = []string{"bread"}
		var buf bytes.Buffer
		err := gob.NewEncoder(&buf).Encode(item)
		assert.NoError(t, err)
		expectedItems[item.Id] = item
		assert.True(t, d.Process(buf.Bytes()))
	}

	actualItems := map[string]*Item{}
	iResult, err := d.Result()
	assert.NoError(t, err)
	result := iResult.([]*Item)
	assert.Equal(t, len(expectedItems), len(result))
	for _, item := range result {
		actualItems[item.Id] = item
	}
	assertdeep.Equal(t, expectedItems, actualItems)
}

func TestGobDecoderNoItems(t *testing.T) {
	d := NewGobDecoder(func() interface{} {
		return &Item{}
	}, func(ch <-chan interface{}) interface{} {
		items := []*Item{}
		for item := range ch {
			items = append(items, item.(*Item))
		}
		return items
	})
	result, err := d.Result()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(result.([]*Item)))
}

func TestGobDecoderError(t *testing.T) {
	item := &Item{}
	item.Id = "Id"
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(item)
	assert.NoError(t, err)
	serialized := buf.Bytes()
	invalid := append([]byte("Hi Mom!"), serialized...)

	d := NewGobDecoder(func() interface{} {
		return &Item{}
	}, func(ch <-chan interface{}) interface{} {
		items := []*Item{}
		for item := range ch {
			items = append(items, item.(*Item))
		}
		return items
	})
	// Process should return true before it encounters an invalid result.
	assert.True(t, d.Process(serialized))
	assert.True(t, d.Process(serialized))
	// Process may return true or false after encountering an invalid value.
	_ = d.Process(invalid)
	for i := 0; i < 250; i++ {
		_ = d.Process(serialized)
	}

	// Result should return error.
	result, err := d.Result()
	assert.Error(t, err)
	assert.Nil(t, result)
}
