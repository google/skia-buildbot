// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package jobsim

import (
	"fmt"
	"testing"

	"github.com/golang/protobuf/jsonpb"
	. "github.com/luci/luci-go/common/testing/assertions"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNormalization(t *testing.T) {
	positive := []struct {
		Name string

		Phrase string
		Parsed *Phrase
	}{
		{
			Name:   "FailureStage",
			Phrase: `{"stages": [{"failure": {"chance": 0.75}}]}`,
			Parsed: &Phrase{
				Stages: []*Stage{{&Stage_Failure{&FailureStage{.75}}}},
			},
		},
	}

	bad := []struct {
		Name string

		Phrase   string
		ParseErr interface{}
		NormErr  interface{}
	}{
		{
			Name:    "FailureStage",
			Phrase:  `{"stages": [{"failure": {"chance": -2}}]}`,
			NormErr: "too small FailureStage chance",
		},
	}

	Convey("TestNormalization", t, func() {
		for _, t := range positive {
			Convey(fmt.Sprintf("good: %s", t.Name), func() {
				p := Phrase{Name: "basic"}
				So(jsonpb.UnmarshalString(t.Phrase, &p), ShouldBeNil)
				So(p.Normalize(), ShouldBeNil)
				if t.Parsed != nil {
					So(t.Parsed, ShouldResemble, t.Parsed)
				}
			})
		}

		for _, t := range bad {
			Convey(fmt.Sprintf("bad: %s", t.Name), func() {
				p := Phrase{Name: "basic"}
				So(jsonpb.UnmarshalString(t.Phrase, &p), ShouldErrLike, t.ParseErr)
				So(p.Normalize(), ShouldErrLike, t.NormErr)
			})
		}
	})
}
