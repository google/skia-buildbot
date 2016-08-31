// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package flagenum

import (
	"encoding/json"
	"flag"
	"fmt"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testValue string

const (
	testFoo    testValue = "test-foo"
	testBar    testValue = "test-bar"
	testEmpty  testValue = "test-empty"
	testQuoted testValue = `test-'"`
)

var (
	testEnum = Enum{
		"foo":      testFoo,
		"bar":      testBar,
		"":         testEmpty,
		`'"`:       testQuoted,
		"failboat": 123,
	}
)

func (tv *testValue) Set(v string) error {
	return testEnum.FlagSet(tv, v)
}

func (tv *testValue) String() string {
	return testEnum.FlagString(tv)
}

func (tv *testValue) UnmarshalJSON(data []byte) error {
	return testEnum.JSONUnmarshal(tv, data)
}

func (tv testValue) MarshalJSON() ([]byte, error) {
	return testEnum.JSONMarshal(tv)
}

type testStruct struct {
	Name string
	A    string
}

var (
	testStructFoo    = testStruct{"testStructFoo", "FOO"}
	testStructBar    = testStruct{"testStructBar", "BAR"}
	testStructQuotes = testStruct{"testStructQuotes", "QUOTES"}

	testStructEnum = Enum{
		"foo": testStructFoo,
		"bar": testStructBar,
		`'"`:  testStructQuotes,
	}
)

func (tv *testStruct) Set(v string) error {
	return testStructEnum.FlagSet(tv, v)
}

func (tv *testStruct) String() string {
	return testStructEnum.FlagString(tv)
}

func (tv *testStruct) UnmarshalJSON(data []byte) error {
	return testStructEnum.JSONUnmarshal(tv, data)
}

func (tv testStruct) MarshalJSON() ([]byte, error) {
	return testStructEnum.JSONMarshal(tv)
}

func TestEnum(t *testing.T) {
	Convey(`A testing enumeration`, t, func() {

		Convey(`String for "testFoo" is "foo".`, func() {
			So(testEnum.GetKey(testFoo), ShouldEqual, "foo")
		})

		Convey(`String for "testQuoted" is ['"].`, func() {
			So(testEnum.GetKey(testQuoted), ShouldEqual, `'"`)
		})

		Convey(`String for unregistered type is "".`, func() {
			So(testEnum.GetKey(0x10), ShouldEqual, "")
		})

		Convey(`Lists choices as: "", '", bar, failboat, foo.`, func() {
			So(testEnum.Choices(), ShouldEqual, `"", '", bar, failboat, foo`)
		})
	})
}

func TestStringEnum(t *testing.T) {
	Convey(`A testing enumeration for a string type`, t, func() {
		Convey(`Panics when deserialized from an incompatible type.`, func() {
			var value testValue
			So(func() { testEnum.setValue(&value, "failboat") }, ShouldPanic)
		})

		Convey(`Panics when attempting to set an unsettable value.`, func() {
			So(func() { testEnum.setValue(123, "foo") }, ShouldPanic)
		})

		Convey(`Binds to an flag field.`, func() {
			var value testValue

			Convey(`When used as a flag`, func() {
				fs := flag.NewFlagSet("test", flag.ContinueOnError)
				fs.Var(&value, "test", "Test flag.")

				Convey(`Sets "value" when set.`, func() {
					err := fs.Parse([]string{"-test", "bar"})
					So(err, ShouldBeNil)
					So(value, ShouldEqual, testBar)
				})

				Convey(`Rejects an invalid enum mapping.`, func() {
					err := fs.Parse([]string{"-test", "baz"})
					So(err, ShouldNotBeNil)
				})
			})

			Convey(`When marshalling to JSON`, func() {
				var s struct {
					Value testValue `json:"value"`
				}

				Convey(`Marshals to JSON as its enumation value.`, func() {
					s.Value = testFoo
					data, err := json.Marshal(&s)
					So(err, ShouldBeNil)
					So(string(data), ShouldEqual, `{"value":"foo"}`)

					Convey(`And unmarshals to its Value.`, func() {
						err := json.Unmarshal(data, &s)
						So(err, ShouldBeNil)
						So(s.Value, ShouldEqual, testFoo)
					})
				})

				Convey(`Unmarshals {"value":"bar"} to "testBar".`, func() {
					err := json.Unmarshal([]byte(`{"value":"bar"}`), &s)
					So(err, ShouldBeNil)
					So(s.Value, ShouldEqual, testBar)
				})

				Convey(`Fails to Unmarshal invalid JSON.`, func() {
					err := json.Unmarshal([]byte(`{"value":123}`), &s)
					So(err, ShouldNotBeNil)
				})
			})
		})
	})
}

func TestStructEnum(t *testing.T) {
	Convey(`A testing enumeration for a struct type`, t, func() {

		Convey(`Panics when attempting to set an unsettable value.`, func() {
			So(func() { testStructEnum.setValue(123, "foo") }, ShouldPanic)
		})

		Convey(`Binds to an flag field.`, func() {
			var value testStruct

			testCases := []struct {
				V string
				S testStruct
			}{
				{"foo", testStructFoo},
				{"bar", testStructBar},
				{`'"`, testStructQuotes},
			}

			Convey(`When used as a flag`, func() {
				fs := flag.NewFlagSet("test", flag.ContinueOnError)
				fs.Var(&value, "test", "Test flag.")

				for _, tc := range testCases {
					Convey(fmt.Sprintf(`Sets "value" to %s when set to [%s].`, tc.S.Name, tc.V), func() {
						err := fs.Parse([]string{"-test", tc.V})
						So(err, ShouldBeNil)
						So(value, ShouldResemble, tc.S)
					})
				}

				Convey(`Rejects an invalid enum mapping.`, func() {
					err := fs.Parse([]string{"-test", "baz"})
					So(err, ShouldNotBeNil)
				})
			})

			Convey(`When marshalling to JSON`, func() {
				var s struct {
					Value testStruct `json:"value"`
				}

				for _, tc := range testCases {
					Convey(fmt.Sprintf(`Marshals %s to JSON as its enumation value, [%s].`,
						tc.S.Name, tc.V), func() {
						s.Value = tc.S
						data, err := json.Marshal(&s)
						So(err, ShouldBeNil)

						mkey, _ := json.Marshal(&tc.V)
						So(string(data), ShouldEqual, fmt.Sprintf(`{"value":%s}`, string(mkey)))

						Convey(`And unmarshals to its Value.`, func() {
							err := json.Unmarshal(data, &s)
							So(err, ShouldBeNil)
							So(s.Value, ShouldResemble, tc.S)
						})
					})
				}

				Convey(`Unmarshals {"value":"bar"} to "testStructBar".`, func() {
					err := json.Unmarshal([]byte(`{"value":"bar"}`), &s)
					So(err, ShouldBeNil)
					So(s.Value, ShouldResemble, testStructBar)
				})

				Convey(`Fails to Unmarshal invalid JSON.`, func() {
					err := json.Unmarshal([]byte(`{"value":123}`), &s)
					So(err, ShouldNotBeNil)
				})
			})
		})
	})
}
