// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package flagenum

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
)

type myType uint

var myTypeEnum = Enum{
	"foo": myType(10),
	"bar": myType(20),
}

var _ flag.Value = (*myType)(nil)

func (val *myType) Set(v string) error {
	return myTypeEnum.FlagSet(val, v)
}

func (val *myType) String() string {
	return myTypeEnum.FlagString(val)
}

func (val myType) MarshalJSON() ([]byte, error) {
	return myTypeEnum.JSONMarshal(val)
}

// Example demonstrates how to use flagenum to create bindings for a custom
// type.
func Example() {
	var value myType

	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&value, "value", "Set the value. Options are: "+myTypeEnum.Choices())
	fs.SetOutput(os.Stdout)

	fs.PrintDefaults()

	// Flag parsing.
	fs.Parse([]string{"-value", "bar"})
	fmt.Printf("Value is: %d\n", value)

	// JSON Marshalling.
	c := struct {
		Value myType `json:"value"`
	}{
		Value: value,
	}
	j, err := json.Marshal(&c)
	if err != nil {
		panic("Failed to marshal JSON.")
	}
	fmt.Printf("JSON is: %s\n", string(j))

	// Output:
	// -value value
	//     	Set the value. Options are: bar, foo
	// Value is: 20
	// JSON is: {"value":"bar"}
}
