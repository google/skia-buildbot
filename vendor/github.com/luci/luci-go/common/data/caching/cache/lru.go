// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package cache

import (
	"container/list"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/luci/luci-go/common/data/text/units"
	"github.com/luci/luci-go/common/isolated"
)

// entry is an entry in the orderedDict.
type entry struct {
	key   isolated.HexDigest
	value units.Size
}

func (e *entry) MarshalJSON() ([]byte, error) {
	// encode as a tuple.
	return json.Marshal([]interface{}{e.key, e.value})
}

func (e *entry) UnmarshalJSON(data []byte) error {
	// decode from tuple.
	var elems []interface{}
	if err := json.Unmarshal(data, &elems); err != nil {
		return fmt.Errorf("invalid entry: %s: %s", err, string(data))
	}
	if len(elems) != 2 {
		return fmt.Errorf("invalid entry: expected 2 items: %s", string(data))
	}
	if key, ok := elems[0].(string); ok {
		e.key = isolated.HexDigest(key)
		if value, ok := elems[1].(float64); ok {
			e.value = units.Size(value)
		} else {
			return fmt.Errorf("invalid entry: expected value to be number: %s", string(data))
		}
	} else {
		return fmt.Errorf("invalid entry: expected key to be string: %s", string(data))
	}
	return nil
}

// orderedDict implements a dict that keeps ordering.
type orderedDict struct {
	ll      *list.List
	entries map[isolated.HexDigest]*list.Element
}

func makeOrderedDict() orderedDict {
	return orderedDict{
		ll:      list.New(),
		entries: map[isolated.HexDigest]*list.Element{},
	}
}

// keys returns the keys in order.
func (o *orderedDict) keys() isolated.HexDigests {
	out := make(isolated.HexDigests, 0, o.length())
	for e := o.ll.Front(); e != nil; e = e.Next() {
		out = append(out, e.Value.(*entry).key)
	}
	return out
}

func (o *orderedDict) pop(key isolated.HexDigest) units.Size {
	if e, hit := o.entries[key]; hit {
		return o.removeElement(e).value
	}
	return 0
}

func (o *orderedDict) popOldest() (isolated.HexDigest, units.Size) {
	if e := o.ll.Back(); e != nil {
		entry := o.removeElement(e)
		return entry.key, entry.value
	}
	return "", 0
}

func (o *orderedDict) removeElement(e *list.Element) *entry {
	o.ll.Remove(e)
	kv := e.Value.(*entry)
	delete(o.entries, kv.key)
	return kv
}

func (o *orderedDict) length() int {
	return o.ll.Len()
}

func (o *orderedDict) pushFront(key isolated.HexDigest, value units.Size) {
	if e, ok := o.entries[key]; ok {
		o.ll.MoveToFront(e)
		e.Value.(*entry).value = value
		return
	}
	o.entries[key] = o.ll.PushFront(&entry{key, value})
}

func (o *orderedDict) pushBack(key isolated.HexDigest, value units.Size) {
	if e, ok := o.entries[key]; ok {
		o.ll.MoveToBack(e)
		e.Value.(*entry).value = value
		return
	}
	o.entries[key] = o.ll.PushBack(&entry{key, value})
}

// serialized returns all the items in order.
func (o *orderedDict) serialized() []entry {
	out := make([]entry, 0, o.length())
	for e := o.ll.Front(); e != nil; e = e.Next() {
		out = append(out, *e.Value.(*entry))
	}
	return out
}

// lruDict is a dictionary that evicts least recently used items. It is a
// higher level than orderedDict.
//
// Designed to be serialized as JSON on disk.
type lruDict struct {
	items orderedDict // ordered key -> value mapping, newest items at the bottom.
	dirty bool        // true if was modified after loading until it is marshaled.
	sum   units.Size  // sum of all the values.
}

func makeLRUDict() lruDict {
	return lruDict{
		items: makeOrderedDict(),
	}
}

func (l *lruDict) IsDirty() bool {
	return l.dirty
}

func (l *lruDict) keys() isolated.HexDigests {
	return l.items.keys()
}

func (l *lruDict) length() int {
	return l.items.length()
}

func (l *lruDict) pop(key isolated.HexDigest) units.Size {
	out := l.items.pop(key)
	l.sum -= out
	l.dirty = true
	return out
}

func (l *lruDict) popOldest() (isolated.HexDigest, units.Size) {
	k, v := l.items.popOldest()
	l.sum -= v
	if k != "" {
		l.dirty = true
	}
	return k, v
}

func (l *lruDict) pushFront(key isolated.HexDigest, value units.Size) {
	l.items.pushFront(key, value)
	l.sum += value
	l.dirty = true
}

func (l *lruDict) touch(key isolated.HexDigest) {
	l.items.pushFront(key, l.items.pop(key))
	l.dirty = true
}

type serializedLRUDict struct {
	Version int     // 1.
	Algo    string  // "sha-1".
	Items   []entry // ordered key -> value mapping in order.
}

func (l *lruDict) MarshalJSON() ([]byte, error) {
	s := &serializedLRUDict{
		Version: 1,
		Algo:    "sha-1",
		Items:   l.items.serialized(),
	}
	// Not strictly true but #closeneough.
	content, err := json.Marshal(s)
	if err == nil {
		l.dirty = false
	}
	return content, err
}

func (l *lruDict) UnmarshalJSON(data []byte) error {
	s := &serializedLRUDict{}
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	if s.Version != 1 {
		return errors.New("invalid lru dict version")
	}
	if s.Algo != "sha-1" {
		return errors.New("invalid lru dict algo")
	}
	l.sum = 0
	for _, e := range s.Items {
		if !e.key.Validate() {
			return fmt.Errorf("invalid entry: %s", e.key)
		}
		l.items.pushBack(e.key, e.value)
		l.sum += e.value
	}
	l.dirty = false
	return nil
}
