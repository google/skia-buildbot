package common

import (
	"reflect"
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestMultiString(t *testing.T) {
	testutils.SmallTest(t)
	m := MultiString{}
	if err := m.Set("alpha"); err != nil {
		t.Errorf("Should not have gotten error: %s", err)
	}
	if err := m.Set("beta,gamma"); err != nil {
		t.Errorf("Should not have gotten error: %s", err)
	}
	if err := m.Set("delta"); err != nil {
		t.Errorf("Should not have gotten error: %s", err)
	}

	expected := MultiString{"alpha", "beta", "gamma", "delta"}
	if !reflect.DeepEqual(expected, m) {
		t.Errorf("Expected %#v, but was %#v", expected, m)
	}
}
