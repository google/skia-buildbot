package iap

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/mockhttpclient"
)

func TestBasics(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)
	assert.Equal(t, "/projects/145247227042/global/backendServices/k8s-be-32071--e03569f20c11b77d", ih.aud)
}

func TestIPinRange(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)
	assert.NoError(t, ih.IPinRange("130.211.1.1:80"))
	assert.NoError(t, ih.IPinRange("35.191.1.1:8080"))
	assert.Error(t, ih.IPinRange("130.211.1.1"))
	assert.Error(t, ih.IPinRange("10.1.1.1:80"))
}

func TestFindKey(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	m := mockhttpclient.NewURLMock()
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar"}`)))
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar"}`)))
	m.MockOnce(IAP_PUBLIC_KEY_URL, mockhttpclient.MockGetDialogue([]byte(`{"foo": "bar", "baz": "quxx"}`)))
	ih.client = m.Client()

	// Make request on empty keys and force an http request.
	key, err := ih.findKey("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", key)
	assert.Len(t, ih.keys, 1)

	// Make request on key that is in cache, so http request.
	key, err = ih.findKey("foo")
	assert.NoError(t, err)
	assert.Equal(t, "bar", key)
	assert.Len(t, ih.keys, 1)

	// Make request for missing key and force an http request, and the key isn't there.
	_, err = ih.findKey("baz")
	assert.Error(t, err)
	assert.Len(t, ih.keys, 1)

	// Make request for missing key and force an http request, and the key is now present.
	key, err = ih.findKey("baz")
	assert.NoError(t, err)
	assert.Equal(t, "quxx", key)
	assert.Len(t, ih.keys, 2)
}

func TestEmail(t *testing.T) {
	h, err := New([]string{}, "145247227042", "k8s-be-32071--e03569f20c11b77d", nil)
	assert.NoError(t, err)
	ih := h.(*iapHandler)

	email, err := ih.getEmail("a.b.c")
	assert.Equal(t, errNotFound, err)

	ih.setEmail("a.b.c", "fred@example.org")

	email, err = ih.getEmail("a.b.c")
	assert.NoError(t, err)
	assert.Equal(t, "fred@example.org", email)

	ih.setEmail("x.y.z", INVALID)
	email, err = ih.getEmail("x.y.z")
	assert.Equal(t, errInvalid, err)
}
