package mockhttpclient

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"reflect"
	"strings"
	"testing"
)

func TestEmptyMock(t *testing.T) {
	urlMock := NewURLMock()
	c := urlMock.Client()

	if _, err := c.Get("http://www.example.com"); err == nil {
		t.Errorf("Should have gotten error for unmocked GET")
	}
}

func getResponseBody(t *testing.T, resp *http.Response) []byte {
	if resp == nil || resp.Body == nil {
		t.Errorf("Response was nil:  %#v", resp)
		return nil
	}
	defer func() {
		err := resp.Body.Close()
		if err != nil {
			// Something horribly horribly bad has happened if our mock body closes and returns an error
			t.Fatalf("Mock response should not have errored on close. %s", err)
		}
	}()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("Problem reading response body: %s", err)
		return nil
	}
	return b
}

func TestGetMockOnce(t *testing.T) {
	urlMock := NewURLMock()

	expectedResponse := []byte("Hello world")
	urlMock.MockOnce("http://www.example.com", MockGetDialogue(expectedResponse))

	if urlMock.Empty() {
		t.Errorf("URLMock should not be empty, but is: %#v", urlMock)
	}

	c := urlMock.Client()
	if resp, err := c.Get("http://www.exampledomain.com"); err == nil {
		t.Errorf("Should have gotten error for unmocked URL, but was %#v", resp)
	}

	if resp, err := c.Get("http://www.example.com"); err != nil {
		t.Errorf("Should not have gotten error for mocked GET: %s", err)
	} else {
		body := getResponseBody(t, resp)
		if !reflect.DeepEqual(expectedResponse, body) {
			t.Errorf("Expected: %#v\n, but was: %#v", expectedResponse, expectedResponse)
		}
	}

	if resp, err := c.Get("http://www.example.com"); err == nil {
		t.Errorf("Should have gotten error for second, unmocked, GET, but was %#v", resp)
	}

	if !urlMock.Empty() {
		t.Errorf("URLMock should now be empty, but is not: %#v", urlMock)
	}
}

func TestGetMockAlways(t *testing.T) {
	expectedResponse := []byte("Hello world")
	c := New(map[string]MockDialogue{
		"http://www.example.com": MockGetDialogue(expectedResponse),
	})
	for i := 0; i < 100; i++ {
		if resp, err := c.Get("http://www.example.com"); err != nil {
			t.Errorf("Should not have gotten error for mocked GET: %s", err)
		} else {
			body := getResponseBody(t, resp)
			if !reflect.DeepEqual(expectedResponse, body) {
				t.Errorf("Expected: %#v\n, but was: %#v", expectedResponse, expectedResponse)
			}
		}
	}
}

func TestPostNotGet(t *testing.T) {
	urlMock := NewURLMock()

	expectedResponse := []byte("Hello world")
	urlMock.MockOnce("http://www.example.com", MockGetDialogue(expectedResponse))

	c := urlMock.Client()
	r := bytes.NewReader([]byte("fizzbuzz"))

	if resp, err := c.Post("http://www.example.com", "text/plain", r); err == nil {
		t.Errorf("Should have gotten error that we POSTed when a GET was mocked, but was %#v", resp)
	}
}

func TestPostWrongBody(t *testing.T) {
	urlMock := NewURLMock()

	expectedResponseBody := []byte("Hello world")
	urlMock.MockOnce("http://www.example.com", MockPostDialogue("text/plain", []byte("password"), expectedResponseBody))

	c := urlMock.Client()
	r := bytes.NewReader([]byte("fizzbuzz"))

	if resp, err := c.Post("http://www.example.com", "text/plain", r); err == nil {
		t.Errorf("Should have gotten error that the request body differed from expected, but was %#v", resp)
	}
}

func TestPostDontCareBody(t *testing.T) {
	urlMock := NewURLMock()

	expectedResponseBody := []byte("Hello world")
	urlMock.MockOnce("http://www.example.com", MockPostDialogue("text/plain", DONT_CARE_REQUEST, expectedResponseBody))

	c := urlMock.Client()
	r := bytes.NewReader([]byte("fizzbuzz"))

	if _, err := c.Post("http://www.example.com", "text/plain", r); err != nil {
		t.Errorf("Should not have gotten error that the request body differed from expected: %s", err)
	}
}

func TestPostWrongType(t *testing.T) {
	urlMock := NewURLMock()

	firstRequestBody := []byte("alpha")
	secondRequestBody := []byte("beta")
	expectedResponseBody := []byte("Hello world")
	urlMock.MockOnce("http://www.example.com", MockPostDialogue("text/plain", firstRequestBody, expectedResponseBody))
	urlMock.MockOnce("http://www.example.com", MockPostDialogue("text/plain", secondRequestBody, expectedResponseBody))

	c := urlMock.Client()

	if _, err := c.Post("http://www.example.com", "text/plain", bytes.NewReader(firstRequestBody)); err != nil {
		t.Errorf("Should not have gotten error for first mocked POST: %s", err)
	}

	if resp, err := c.Post("http://www.example.com", "application/json", bytes.NewReader(secondRequestBody)); err == nil {
		t.Errorf("Should have gotten error for second POST of wrong type, but was %#v", resp)
	} else if !strings.Contains(err.Error(), `expected "text/plain", but was "application/json"`) {
		t.Errorf("The wrong error was shown: %s", err)
	}
}
