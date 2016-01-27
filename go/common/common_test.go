package common

import (
	"reflect"
	"testing"
)

func TestMultiString(t *testing.T) {
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
