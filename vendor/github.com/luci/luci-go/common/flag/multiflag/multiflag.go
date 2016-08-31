// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package multiflag is a package providing a flag.Value implementation capable
// of switching between multiple registered sub-flags, each of which have their
// own set of parameter flags.
//
// Example
//
// This can be used to construct complex option flags. For example:
//   -backend mysql,address="192.168.1.1",port="12345"
//   -backend sqlite3,path="/path/to/database"
//
// In this example, a MultiFlag is defined and bound to the option name,
// "backend". This MultiFlag has (at least) two registered Options:
//   1) mysql, whose FlagSet binds "address" and "port" options.
//   2) sqlite3, whose FlagSet binds "path".
//
// The MultiFlag Option that is selected (e.g., "mysql") has the remainder of
// its option string parsed into its FlagSet, populating its "address" and
// "port" parameters. If "sqlite3" is selected, the remainder of the option
// string would be parsed into its FlagSet, which hosts the "path" parameter.
package multiflag

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/luci/luci-go/common/flag/nestedflagset"
)

// OptionDescriptor is a collection of common Option properties.
type OptionDescriptor struct {
	Name        string
	Description string

	Pinned bool
}

// Option is a single option entry in a MultiFlag. An Option is responsible
// for parsing a FlagSet from an option string.
type Option interface {
	Descriptor() *OptionDescriptor

	PrintHelp(io.Writer)
	Run(string) error // Parses the Option from a configuration string.
}

// MultiFlag is a flag.Value-like object that contains multiple sub-options.
// Each sub-option presents itself as a flag.FlagSet. The sub-option that gets
// selected will have its FlagSet be evaluated against the accompanying options.
//
// For example, one can construct flag that, depending on its options, chooses
// one of two sets of sub-properties:
//
//   -myflag foo,foovalue=123
//   -myflag bar,barvalue=456,barothervalue="hello"
//
// "myflag" is the name of the MultiFlag's top-level flag, as registered with a
// flag.FlagSet. The first token in the flag's value selects which Option should
// be configured. If "foo" is configured, the remaining configuration is parsed
// by the "foo" Option's FlagSet, and the equivalent is true for "bar".
type MultiFlag struct {
	Description string
	Options     []Option
	Output      io.Writer // Output writer, or nil to use STDERR.

	// The selected Option, populated after Parsing.
	Selected Option
}

var _ flag.Value = (*MultiFlag)(nil)

// GetOutput returns the output Writer used for help output.
func (mf *MultiFlag) GetOutput() io.Writer {
	if w := mf.Output; w != nil {
		return w
	}
	return os.Stderr
}

// Parse applies a value string to a MultiFlag.
//
// For example, if the value string is:
// foo,option1=test
//
// Parse will identify the MultiFlag option named "foo" and have it parse the
// string, "option1=test".
func (mf *MultiFlag) Parse(value string) error {
	option, params := parseOptionParams(value)
	if len(option) == 0 {
		return errors.New("Option cannot be empty.")
	}

	mf.Selected = mf.GetOptionFor(option)
	if mf.Selected == nil {
		return fmt.Errorf("Invalid option: %v", option)
	}
	return mf.Selected.Run(params)
}

// Set implements flag.Value
func (mf *MultiFlag) Set(value string) error {
	return mf.Parse(value)
}

// String implements flag.Value
func (mf *MultiFlag) String() string {
	return strings.Join(mf.OptionNames(), ",")
}

// GetOptionFor returns the Option associated with the specified name, or nil
// if one isn't defined.
func (mf *MultiFlag) GetOptionFor(name string) Option {
	for _, option := range mf.Options {
		if option.Descriptor().Name == name {
			return option
		}
	}
	return nil
}

// OptionNames returns a list of the option names registered with a MultiFlag.
func (mf MultiFlag) OptionNames() []string {
	optionNames := make([]string, 0, len(mf.Options))
	for _, opt := range mf.Options {
		optionNames = append(optionNames, opt.Descriptor().Name)
	}
	return optionNames
}

// PrintHelp prints a formatted help string for a MultiFlag. This help string
// details the Options registered with the MultiFlag.
func (mf *MultiFlag) PrintHelp() error {
	descriptors := make(optionDescriptorSlice, len(mf.Options))
	for idx, opt := range mf.Options {
		descriptors[idx] = opt.Descriptor()
	}
	sort.Sort(descriptors)

	fmt.Fprintln(mf.Output, mf.Description)
	return writeAlignedOptionDescriptors(mf.Output, []*OptionDescriptor(descriptors))
}

// FlagOption is an implementation of Option that is describes a single
// nestedflagset option. This option has sub-properties that
type FlagOption struct {
	Name        string
	Description string
	Pinned      bool

	flags nestedflagset.FlagSet
}

var _ Option = (*FlagOption)(nil)

// IsPinned implements Option.
func (o *FlagOption) IsPinned() bool {
	return o.Pinned
}

// Descriptor implements Option.
func (o *FlagOption) Descriptor() *OptionDescriptor {
	return &OptionDescriptor{
		Name:        o.Name,
		Description: o.Description,
		Pinned:      o.Pinned,
	}
}

// PrintHelp implements Option.
func (o *FlagOption) PrintHelp(output io.Writer) {
	flags := o.Flags()
	flags.SetOutput(output)
	flags.PrintDefaults()
}

// Flags returns this Option's nested FlagSet for configuration.
func (o *FlagOption) Flags() *flag.FlagSet {
	return &o.flags.F
}

// Run implements Option.
func (o *FlagOption) Run(value string) error {
	if err := o.flags.Parse(value); err != nil {
		return err
	}
	return nil
}

// optionDescriptorSlice is a slice of Option interfaces.
type optionDescriptorSlice []*OptionDescriptor

var _ sort.Interface = optionDescriptorSlice(nil)

// Implement sort.Interface
func (s optionDescriptorSlice) Len() int {
	return len(s)
}

// Implement sort.Interface
func (s optionDescriptorSlice) Less(i, j int) bool {
	// Pinned items are always less than unpinned items.
	if s[i].Pinned {
		if !s[j].Pinned {
			return true
		}
	} else if s[j].Pinned {
		return false
	}

	return s[i].Name < s[j].Name
}

// Implement sort.Interface
func (s optionDescriptorSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

// Option implementation that displays help for a configured MultiFlag.
type helpOption struct {
	mf *MultiFlag
}

var helpOptionDescriptor = OptionDescriptor{
	Name:        "help",
	Description: `Displays this help message. Can be run as "help,<option>" to display help for an option.`,
	Pinned:      true,
}

// HelpOption instantiates a new Option instance that prints a help string when
// parsed.
func HelpOption(mf *MultiFlag) Option {
	return &helpOption{mf}
}

func (o *helpOption) Descriptor() *OptionDescriptor {
	return &helpOptionDescriptor
}

func (o *helpOption) PrintHelp(io.Writer) {}

func (o *helpOption) Run(value string) error {
	if value == "" {
		return o.mf.PrintHelp()
	}

	output := o.mf.GetOutput()
	opt := o.mf.GetOptionFor(value)
	if opt != nil {
		desc := opt.Descriptor()
		fmt.Fprintf(output, "Help for '%s': %s\n", desc.Name, desc.Description)
		opt.PrintHelp(output)
		return nil
	}

	fmt.Fprintf(output, "Unknown option '%s'\n", value)
	return nil
}

// parseOptionParams parses an input parameter into its option name (first
// component) and optional parameter data.
//
// For example:
// "option" => option="option", params=""
// "option,params,foo,bar" => option="option", params="params,foo,bar"
func parseOptionParams(value string) (option, params string) {
	// Strip off the first component; use this as the option name.
	idx := strings.IndexRune(value, ',')

	if idx == -1 {
		option, params = value, ""
	} else {
		option, params = value[:idx], value[(idx+1):]
	}
	return
}

// writeAlignedOptionDescriptors writes help entries for a series of Options.
func writeAlignedOptionDescriptors(w io.Writer, descriptors []*OptionDescriptor) error {
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)

	for _, desc := range descriptors {
		fmt.Fprintf(tw, "%s\t%s\n", desc.Name, desc.Description)
	}

	if err := tw.Flush(); err != nil {
		return err
	}
	return nil
}
