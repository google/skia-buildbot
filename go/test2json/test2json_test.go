package test2json

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestEventStreamFail(t *testing.T) {
	unittest.MediumTest(t)
	RunTestAndCompare(t, EVENTS_FAIL, CONTENT_FAIL)
}

func TestEventStreamPass(t *testing.T) {
	unittest.MediumTest(t)
	RunTestAndCompare(t, EVENTS_PASS, CONTENT_PASS)
}

func TestEventStreamSkip(t *testing.T) {
	unittest.MediumTest(t)
	RunTestAndCompare(t, EVENTS_SKIP, CONTENT_SKIP)
}
