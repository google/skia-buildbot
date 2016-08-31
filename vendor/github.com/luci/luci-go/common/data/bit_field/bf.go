// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

//go:generate go run ./tool/gen.go

package bit_field

import (
	"bytes"
	"encoding"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/luci/gae/service/datastore"
)

// BitField is a luci/gae-serializable bit field implementation. It should
// be nice and fast for non-AppEngine use as well.
//
// You should construct a new one with bf.Make, rather than by direct
// construction.
type BitField struct {
	// data is the actual bits data. rightmost bit of 0th byte is the first bit.
	data []byte
	size uint32
}

var _ interface {
	// Compatible with luci/gae/service/datastore
	datastore.PropertyConverter

	// Compatible with gob
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
} = (*BitField)(nil)

// MarshalBinary implements encoding.BinaryMarshaller
func (bf *BitField) MarshalBinary() ([]byte, error) {
	ret := make([]byte, binary.MaxVarintLen32+len(bf.data))
	n := binary.PutUvarint(ret, uint64(bf.size))
	n += copy(ret[n:], bf.data)
	return ret[:n], nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler
func (bf *BitField) UnmarshalBinary(bs []byte) error {
	buf := bytes.NewBuffer(bs)
	n, err := binary.ReadUvarint(buf)
	if err != nil {
		return err
	}
	if n > math.MaxUint32 {
		return fmt.Errorf("encoded number is out of range: %d > %d", n, uint32(math.MaxUint32))
	}

	size := uint32(n)
	if size == 0 {
		bf.size = 0
		bf.data = nil
		return nil
	}
	data := buf.Bytes()
	if uint32(len(data)) != (size+8-1)>>3 {
		return fmt.Errorf("mismatched size (%d) v. byte count (%d)", size, len(data))
	}

	bf.size = size
	bf.data = data
	return nil
}

// ToProperty implements datastore.PropertyConverter
func (bf *BitField) ToProperty() (datastore.Property, error) {
	bs, err := bf.MarshalBinary()
	return datastore.MkPropertyNI(bs), err
}

// FromProperty implements datastore.PropertyConverter
func (bf *BitField) FromProperty(p datastore.Property) error {
	bin, ok := p.Value().([]byte)
	if !ok {
		return fmt.Errorf("expected %s, got %s", datastore.PTBytes, p.Type())
	}
	return bf.UnmarshalBinary(bin)
}

// Reset resets this BitField to the 'empty' (size-0) state.
func (bf *BitField) Reset() {
	bf.data = nil
	bf.size = 0
}

// Make creates a new BitField.
func Make(size uint32) BitField {
	if size == 0 {
		return BitField{}
	}
	return BitField{
		data: make([]byte, (size+8-1)>>3),
		size: size,
	}
}

// Size returns the number of bits which this BitField can hold.
func (bf BitField) Size() uint32 {
	return bf.size
}

// Set turns the given bit to true, regardless of its previous value. Will panic
// if idx >= Size().
func (bf BitField) Set(idx uint32) {
	if idx >= bf.size {
		panic(fmt.Errorf("cannot set bit %d in BitField of size %d", idx, bf.size))
	}
	bf.data[idx>>3] |= 1 << (idx % 8)
}

// Clear turns the given bit to false, regardless of its previous value. Will
// panic if idx >= Size().
func (bf BitField) Clear(idx uint32) {
	if idx >= bf.size {
		panic(fmt.Errorf("cannot clear bit %d in BitField of size %d", idx, bf.size))
	}
	bf.data[idx>>3] &^= 1 << (idx % 8)
}

// All returns true iff all of the bits are equal to `val`.
func (bf BitField) All(val bool) bool {
	targ := bf.size
	if !val {
		targ = 0
	}
	return bf.CountSet() == targ
}

// CountSet returns the number of true bits.
func (bf BitField) CountSet() (ret uint32) {
	for _, b := range bf.data {
		if b != 0 {
			ret += uint32(bitCount[b])
		}
	}
	return ret
}

// IsSet returns the value of a given bit.
func (bf BitField) IsSet(idx uint32) bool {
	return idx < bf.size && ((bf.data[idx>>3] & (1 << (idx % 8))) != 0)
}
