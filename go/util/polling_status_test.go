package util

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/mockhttpclient"
)

var urlMap = map[string][]byte{}

func TestIntPollingStatus(t *testing.T) {
	i := 0
	tc := []int{0, 10, -35}
	duration := 100 * time.Millisecond
	ps, err := NewIntPollingStatus(func(v interface{}) error {
		if i >= len(tc) {
			return fmt.Errorf("Reached the end of test data.")
		}
		*v.(*intValue) = intValue{tc[i]}
		return nil
	}, duration)
	assert.Nil(t, err)

	time.Sleep(duration / 2)
	for j, v := range tc {
		bytes, err := json.Marshal(ps)
		assert.Nil(t, err)

		assert.Equal(t, []byte(strconv.FormatInt(int64(v), 10)), bytes)
		i = j + 1
		time.Sleep(duration)
	}
	ps.Stop()
}

func TestJSONPollingStatus(t *testing.T) {
	// Prepare test data.
	url := "http://my.json.endpoint"
	obj := map[string]string{
		"a": "1",
		"b": "2",
	}
	v := map[string]string{}
	bytes, err := json.Marshal(obj)
	assert.Nil(t, err)

	// Mock out the HTTP client.
	httpClient := mockhttpclient.New(map[string][]byte{url: bytes})

	// Create a PollingStatus and verify that it obtains the expected result.
	ps, err := NewJSONPollingStatus(&v, url, time.Second, httpClient)
	assert.Nil(t, err)
	assert.True(t, reflect.DeepEqual(obj, v))

	ps.Stop()
}
