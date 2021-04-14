package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestElementsSkStylesheetsFromTsImport_NotAnElementsSkImport_ReturnsEmptySlice(t *testing.T) {
	unittest.SmallTest(t)
	assert.Empty(t, elementsSkStylesheetsFromTsImport("path/to/foo"))
}

func TestElementsSkStylesheetsFromTsImport_ModuleWithNoDeps_ReturnsOneStylesheet(t *testing.T) {
	unittest.SmallTest(t)
	expected := []string{"elements-sk/collapse-sk/collapse-sk.scss"}
	assert.ElementsMatch(t, expected, elementsSkStylesheetsFromTsImport("elements-sk/collapse-sk"))
	assert.ElementsMatch(t, expected, elementsSkStylesheetsFromTsImport("elements-sk/collapse-sk/collapse-sk"))
}

func TestElementsSkStylesheetsFromTsImport_ModuleWithDeps_ReturnsMultipleStylesheets(t *testing.T) {
	unittest.SmallTest(t)
	expected := []string{
		"elements-sk/error-toast-sk/error-toast-sk.scss",
		"elements-sk/toast-sk/toast-sk.scss",
	}
	assert.ElementsMatch(t, expected, elementsSkStylesheetsFromTsImport("elements-sk/error-toast-sk"))
}
