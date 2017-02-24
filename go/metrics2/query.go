package metrics2

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
)

const (
	BASE_URL = "https://prom.skia.org/api/v1/query"
)

type PollingQuery struct {
	c     *http.Client
	url   string
	mutex sync.Mutex
	value float64
}

func NewPollingQuery(c *http.Client, query string, period time.Duration) (*PollingQuery, error) {
	// Construct the URL.
	u, err := url.Parse(BASE_URL)
	if err != nil {
		return nil, fmt.Errorf("Invalid base url %q: %s", BASE_URL, err)
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	ret := &PollingQuery{
		c:   c,
		url: u.String(),
	}
	err = ret.step()
	if err != nil {
		return nil, err
	}

	go func(p *PollingQuery, query string) {
		for _ = range time.Tick(period) {
			if err := p.step(); err != nil {
				sklog.Errorf("Failed polling query %q: %s", query, err)
			}
		}
	}(ret, query)
	return ret, nil
}

func (p *PollingQuery) Get() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.value
}

type indResult struct {
	Metric map[string]string `json:"metric"`
	Value  []interface{}     `json:"value"`
}

type data struct {
	ResultType string      `json:"resultType"`
	Result     []indResult `json:"result"`
}

type queryResponse struct {
	Status string `json:"status"`
	Data   data   `json:"status"`
}

func (p *PollingQuery) step() error {
	resp, err := p.c.Get(p.url)
	if err != nil {
		return fmt.Errorf("Failed to make initial request: %s", err)
	}
	q := &queryResponse{}
	if err := json.NewDecoder(resp.Body).Decode(q); err != nil {
		return fmt.Errorf("Failed to decode response: %s", err)
	}
	if q.Status != "success" {
		return fmt.Errorf("Query failed %q: %s", q.Status, err)
	}
	if q.Data.ResultType != "vector" {
		return fmt.Errorf("Query response wrong type %q: %s", q.Data.ResultType, err)
	}
	if len(q.Data.Result) != 1 {
		return fmt.Errorf("Query response wrong length %d: %s", len(q.Data.Result), err)
	}
	s, ok := q.Data.Result[0].Value[1].(string)
	if !ok {
		return fmt.Errorf("Query response wrong value type.")
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("Query response parse failed %q: %s", s, err)
	}
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.value = f

	return nil
}
