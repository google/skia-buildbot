package readme_chromium

/*
	This package contains utilities for working with README.chromium files.
*/

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"go.skia.org/infra/go/skerr"
)

const FileName = "README.chromium"

type UpdateMechanism string

const (
	UpdateMechanism_Autoroll       UpdateMechanism = "Autoroll"
	UpdateMechanism_Manual         UpdateMechanism = "Manual"
	UpdateMechanism_Static         UpdateMechanism = "Static"
	UpdateMechanism_StaticHardFork UpdateMechanism = "Static.HardFork"
)

// ReadmeChromiumFile represents the contents of a README.chromium file.
type ReadmeChromiumFile struct {
	originalContentLines []string
	fields               []*field

	Name                     string
	ShortName                string
	URL                      string
	Version                  string
	Date                     string
	Revision                 string
	UpdateMechanism          UpdateMechanism
	License                  string
	LicenseFile              string
	Shipped                  bool
	SecurityCritical         bool
	LicenseAndroidCompatible bool
	CPEPrefix                string

	// Note: according to the template[1] we've skipped Description and
	// LocalModifications. These are multiline sections which may or may not be
	// present, which makes it difficult to parse them. We'll revisit if
	// necessary.
	// [1] https://chromium.googlesource.com/chromium/src.git/+/main/third_party/README.chromium.template
}

// Parse and return a ReadmeChromiumFile file with the given contents.
func Parse(content string) (*ReadmeChromiumFile, error) {
	rv := &ReadmeChromiumFile{
		originalContentLines: strings.Split(content, "\n"),
		fields:               makeFields(),
	}
	val := reflect.ValueOf(rv).Elem()
	for _, f := range rv.fields {
		regex := regexForField(f)
		for lineNo, line := range rv.originalContentLines {
			matches := regex.FindStringSubmatchIndex(line)
			// [startOfOverallMatch, endOfOverallMatch, startOfSubMatch, endOfSubMatch]
			if len(matches) == 4 {
				f.LineNo = lineNo
				f.StartIndex = matches[2]
				f.EndIndex = matches[3]
				break
			}
		}

		if f.Required && f.LineNo == 0 && f.StartIndex == 0 && f.EndIndex == 0 {
			return nil, skerr.Fmt("failed to find field %q using regex %q", f.Name, regex.String())
		}

		// Assign the field to the return value.
		strVal := string(rv.originalContentLines[f.LineNo][f.StartIndex:f.EndIndex])
		field := val.FieldByName(strings.ReplaceAll(f.Name, " ", ""))
		if !field.IsValid() {
			return nil, skerr.Fmt("field %q is invalid", f.Name)
		}
		if !field.CanSet() {
			return nil, skerr.Fmt("field %q cannot be set", f.Name)
		}
		if field.Kind() == reflect.String {
			field.SetString(strVal)
		} else if field.Kind() == reflect.Bool {
			field.SetBool(strVal == "yes")
		}
	}
	return rv, nil
}

// NewContent returns the updated README.chromium file content, incorporating
// any changes to the fields of the ReadmeChromiumFile.
func (file *ReadmeChromiumFile) NewContent() (string, error) {
	val := reflect.ValueOf(file).Elem()
	newContentLines := make([]string, 0, len(file.originalContentLines))
	for _, line := range file.originalContentLines {
		newContentLines = append(newContentLines, line)
	}

	for _, f := range file.fields {
		if f.LineNo == 0 && f.StartIndex == 0 && f.EndIndex == 0 {
			// This field isn't set, so we can't update it.
			continue
		}

		oldValue := newContentLines[f.LineNo][f.StartIndex:f.EndIndex]

		field := val.FieldByName(strings.ReplaceAll(f.Name, " ", ""))
		if !field.IsValid() {
			return "", skerr.Fmt("field %q is invalid", f.Name)
		}
		newValue := field.String()
		if field.Kind() == reflect.Bool {
			if field.Bool() {
				newValue = "yes"
			} else {
				newValue = "no"
			}
		}

		newContentLines[f.LineNo] = strings.Replace(newContentLines[f.LineNo], oldValue, newValue, 1)
	}
	return strings.Join(newContentLines, "\n"), nil
}

// dependencyDividerRegex is used to find the separator between sections in a
// README.chromium file containing metadata about multiple dependencies. Note
// that we use a regex for parsing, in case a file contains a slightly different
// number of dashes.
var dependencyDividerRegex = regexp.MustCompile(`(?m)^-+ DEPENDENCY DIVIDER -+$`)

// dependencyDivider is a separator between sections in a README.chromium file
// containing metadata about multiple dependencies. Note that we use a regex for
// parsing, in case a file contains a slightly different number of dashes.
const dependencyDivider = "-------------------- DEPENDENCY DIVIDER --------------------"

// ParseMulti parses at least one ReadmeChromiumFile from the given contents.
func ParseMulti(contents string) ([]*ReadmeChromiumFile, error) {
	sections := dependencyDividerRegex.Split(contents, -1)
	rv := make([]*ReadmeChromiumFile, 0, len(sections))
	for _, section := range sections {
		f, err := Parse(section)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, f)
	}
	return rv, nil
}

// WriteMulti returns the content of multiple ReadmeChromiumFiles.
func WriteMulti(files []*ReadmeChromiumFile) (string, error) {
	allContents := make([]string, 0, len(files))
	for _, f := range files {
		content, err := f.NewContent()
		if err != nil {
			return "", skerr.Wrap(err)
		}
		allContents = append(allContents, content)
	}
	return strings.Join(allContents, dependencyDivider), nil
}

type field struct {
	Name       string
	LineNo     int
	StartIndex int
	EndIndex   int
	Required   bool
}

func makeFields() []*field {
	return []*field{
		{Name: "Name", Required: true},
		{Name: "Short Name"},
		{Name: "URL", Required: true},
		{Name: "Version", Required: true},
		{Name: "Date"},
		{Name: "Revision"},
		{Name: "Update Mechanism", Required: true},
		{Name: "License", Required: true},
		{Name: "License File"},
		{Name: "Shipped"},
		{Name: "Security Critical"},
		{Name: "License Android Compatible"},
		{Name: "CPEPrefix"},
	}
}

func regexForField(f *field) *regexp.Regexp {
	return regexp.MustCompile(fmt.Sprintf(`^%s:\s+(.+)$`, f.Name))
}
