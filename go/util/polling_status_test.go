package util

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
)

var urlMap = map[string][]byte{}

func TestPollingStatus(t *testing.T) {
	i := 0
	tc := []int64{0, 10, -35}
	duration := 100 * time.Millisecond
	ps, err := NewPollingStatus(func() (interface{}, error) {
		if i >= len(tc) {
			return 0, fmt.Errorf("Reached the end of test data.")
		}
		return tc[i], nil
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
