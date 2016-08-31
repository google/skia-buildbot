// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package multiflag

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
)

type testMultiFlag struct {
	MultiFlag

	S string
	I int
	B bool
}

func (of *testMultiFlag) newOption(name, description string) Option {
	o := &FlagOption{Name: name, Description: description}

	flags := o.Flags()
	flags.StringVar(&of.S, "string-var", "", "A string variable.")
	flags.IntVar(&of.I, "int-var", 123, "An integer variable.")
	flags.BoolVar(&of.B, "bool-var", false, "A boolean variable.")

	// Set our option name.
	return o
}

// A Writer implementation that controllably fails.
type failWriter struct {
	Max   int   // When failing, never "write" more than this many bytes.
	Error error // The error to return when used to write.
}

// Implements io.Writer.
func (w *failWriter) Write(data []byte) (int, error) {
	size := len(data)
	if w.Error != nil && size > w.Max {
		size = w.Max
	}
	return size, w.Error
}

func TestParsing(t *testing.T) {
	Convey("Given empty MultiFlag", t, func() {
		of := &testMultiFlag{}

		Convey("Its option list should be empty", func() {
			So(of.OptionNames(), ShouldResemble, []string{})
		})

		Convey(`Parsing an option spec with an empty option value should fail.`, func() {
			So(of.Parse(`,params`), ShouldNotBeNil)
		})
	})

	Convey("Given Flags with two option values", t, func() {
		of := &testMultiFlag{}
		of.Options = []Option{
			of.newOption("foo", "Test option 'foo'."),
			of.newOption("bar", "Test option 'bar'."),
		}

		Convey("Its option list should be: ['foo', 'bar'].", func() {
			So(of.OptionNames(), ShouldResemble, []string{"foo", "bar"})
		})

		Convey(`When parsing 'foo,string-var="hello world"'`, func() {
			err := of.Parse(`foo,string-var="hello world",bool-var=true`)
			So(err, ShouldBeNil)

			Convey("The option, 'foo', should be selected.", func() {
				So(of.Selected, ShouldNotBeNil)
				So(of.Selected.Descriptor().Name, ShouldEqual, "foo")
			})

			Convey(`The value of 'string-var' should be, "hello world".`, func() {
				So(of.S, ShouldEqual, "hello world")
			})

			Convey(`The value of 'int-var' should be default (123)".`, func() {
				So(of.I, ShouldEqual, 123)
			})

			Convey(`The value of 'bool-var' should be "true".`, func() {
				So(of.B, ShouldEqual, true)
			})
		})
	})
}

func TestHelp(t *testing.T) {
	Convey(`A 'MultiFlag' instance`, t, func() {
		of := &MultiFlag{}

		Convey(`Uses os.Stderr for output.`, func() {
			So(of.GetOutput(), ShouldEqual, os.Stderr)
		})

		Convey(`Configured with a simple FlagOption with flags`, func() {
			opt := &FlagOption{
				Name:        "foo",
				Description: "An option, 'foo'.",
			}
			opt.Flags().String("bar", "", "An option, 'bar'.")
			of.Options = []Option{opt}

			Convey(`Should successfully parse "foo" with no flags.`, func() {
				So(of.Parse(`foo`), ShouldBeNil)
			})

			Convey(`Should successfully parse "foo" with a "bar" flag.`, func() {
				So(of.Parse(`foo,bar="Hello!"`), ShouldBeNil)
			})

			Convey(`Should fail to parse a non-existent flag.`, func() {
				So(of.Parse(`foo,baz`), ShouldNotBeNil)
			})
		})
	})

	Convey(`Given a testMultiFlag configured with 'nil' output.`, t, func() {
		of := &testMultiFlag{}

		Convey(`Should default to os.Stderr`, func() {
			So(of.GetOutput(), ShouldEqual, os.Stderr)
		})
	})

	Convey("Given Flags with two options, one of which is 'help'", t, func() {
		var buf bytes.Buffer
		of := &testMultiFlag{}
		of.Output = &buf
		of.Options = []Option{
			HelpOption(&of.MultiFlag),
			of.newOption("foo", "Test option 'foo'."),
		}

		correctHelpString := `
help  Displays this help message. Can be run as "help,<option>" to display help for an option.
foo   Test option 'foo'.
`

		Convey("Should print a correct help string", func() {
			err := of.PrintHelp()
			So(err, ShouldBeNil)

			So(buf.String(), ShouldEqual, correctHelpString)
		})

		Convey(`Should fail to print a help string when the writer fails.`, func() {
			w := &failWriter{}
			w.Error = errors.New("Fail.")
			of.Output = w
			So(of.PrintHelp(), ShouldNotBeNil)
		})

		Convey(`Should parse a request for a specific option's help string.`, func() {
			err := of.Parse(`help,foo`)
			So(err, ShouldBeNil)

			Convey(`And should print help for that option.`, func() {
				correctOptionHelpString := `Help for 'foo': Test option 'foo'.
  -bool-var
    	A boolean variable.
  -int-var int
    	An integer variable. (default 123)
  -string-var string
    	A string variable.
`
				So(buf.String(), ShouldEqual, correctOptionHelpString)
			})
		})

		Convey("Should parse the 'help' option", func() {
			err := of.Parse(`help`)
			So(err, ShouldBeNil)

			Convey("Should print the correct help string in response.", func() {
				So(buf.String(), ShouldEqual, correctHelpString)
			})
		})

		Convey("Should parse 'help,junk=1'", func() {
			err := of.Parse(`help,junk=1`)
			So(err, ShouldBeNil)

			Convey(`Should notify the user that "junk=1" is not an option.`, func() {
				So(buf.String(), ShouldEqual, "Unknown option 'junk=1'\n")
			})
		})
	})
}

func TestHelpItemSlice(t *testing.T) {
	Convey(`Given a slice of testOption instances`, t, func() {
		options := optionDescriptorSlice{
			&OptionDescriptor{
				Name:        "a",
				Description: "An unpinned help item",
				Pinned:      false,
			},
			&OptionDescriptor{
				Name:        "b",
				Description: "Another unpinned help item",
				Pinned:      false,
			},
			&OptionDescriptor{
				Name:        "c",
				Description: "A pinned help item",
				Pinned:      true,
			},
			&OptionDescriptor{
				Name:        "d",
				Description: "Another pinned help item",
				Pinned:      true,
			},
		}
		sort.Sort(options)

		Convey(`The options should be sorted: c, b, a, b`, func() {
			var names []string
			for _, opt := range options {
				names = append(names, opt.Name)
			}
			So(names, ShouldResemble, []string{"c", "d", "a", "b"})
		})
	})
}

func TestFlagParse(t *testing.T) {
	Convey("Given a MultiFlag with one option, 'foo'", t, func() {
		of := &testMultiFlag{}
		of.Options = []Option{
			of.newOption("foo", "Test option 'foo'."),
		}

		Convey("When configured as an output to a Go flag.MultiFlag", func() {
			gof := &flag.FlagSet{}
			gof.Var(of, "option", "Single line option")

			Convey(`Should parse '-option foo,string-var="hello world",int-var=1337'`, func() {
				err := gof.Parse([]string{"-option", `foo,string-var="hello world",int-var=1337"`})
				So(err, ShouldBeNil)

				Convey("Should parse out 'foo' as the option", func() {
					So(of.Selected.Descriptor().Name, ShouldEqual, "foo")
				})

				Convey(`Should parse out 'string-var' as "hello world".`, func() {
					So(of.S, ShouldEqual, "hello world")
				})

				Convey(`Should parse out 'int-var' as '1337'.`, func() {
					So(of.I, ShouldEqual, 1337)
				})
			})

			Convey(`When parsing 'bar'`, func() {
				err := gof.Parse([]string{"-option", "bar"})
				So(err, ShouldNotBeNil)
			})
		})
	})
}

func ExampleMultiFlag() {
	// Setup multiflag.
	param := ""
	deprecated := FlagOption{
		Name:        "deprecated",
		Description: "The deprecated option.",
	}
	deprecated.Flags().StringVar(&param, "param", "", "String parameter.")

	beta := FlagOption{Name: "beta", Description: "The new option, which is still beta."}
	beta.Flags().StringVar(&param, "param", "", "Beta string parameter.")

	mf := MultiFlag{
		Description: "My test MultiFlag.",
		Output:      os.Stdout,
	}
	mf.Options = []Option{
		HelpOption(&mf),
		&deprecated,
		&beta,
	}

	// Install the multiflag as a flag in "flags".
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Var(&mf, "multiflag", "Multiflag option.")

	// Parse flags (help).
	cmd := []string{"-multiflag", "help"}
	fmt.Println("Selecting help option:", cmd)
	if err := fs.Parse(cmd); err != nil {
		panic(err)
	}

	// Parse flags (param).
	cmd = []string{"-multiflag", "beta,param=Sup"}
	fmt.Println("Selecting beta option:", cmd)
	if err := fs.Parse(cmd); err != nil {
		panic(err)
	}
	fmt.Printf("Option [%s], parameter: [%s].\n", mf.Selected.Descriptor().Name, param)

	// Output:
	// Selecting help option: [-multiflag help]
	// My test MultiFlag.
	// help        Displays this help message. Can be run as "help,<option>" to display help for an option.
	// beta        The new option, which is still beta.
	// deprecated  The deprecated option.
	// Selecting beta option: [-multiflag beta,param=Sup]
	// Option [beta], parameter: [Sup].
}
