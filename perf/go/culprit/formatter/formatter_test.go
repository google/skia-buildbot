package formatter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	"go.skia.org/infra/perf/go/config"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func TestFormatting_HappyPath(t *testing.T) {
	commitUrlTemplate := "https://skia.googlesource.com/skia/+log/%s"
	cfg := &config.InstanceConfig{
		IssueTrackerConfig: config.IssueTrackerConfig{},
	}
	culprit := &pb.Culprit{
		Commit: &pb.Commit{
			Host:     "chromium.googlesource.com",
			Project:  "chromium/src",
			Ref:      "refs/head/main",
			Revision: "revision123",
		},
	}
	subscription := &sub_pb.Subscription{
		Name: "test subscription",
	}
	f, err := NewMarkdownFormatter(commitUrlTemplate, cfg)

	require.NoError(t, err)
	subject, body, err := f.GetCulpritSubjectAndBody(context.Background(), culprit, subscription)
	assert.True(t, strings.Contains(string(subject), subscription.Name))
	assert.True(t, strings.Contains(string(body), fmt.Sprintf(commitUrlTemplate, culprit.Commit.Revision)))
	assert.Nil(t, err)
}

func TestFormatting_Report_NoConfig(t *testing.T) {
	commitUrlTemplate := "https://skia.googlesource.com/skia/+log/%s"
	cfg := &config.InstanceConfig{
		URL:                "test-url",
		IssueTrackerConfig: config.IssueTrackerConfig{},
	}
	subscription := &sub_pb.Subscription{
		Name: "Subscription_Test",
	}
	f, err := NewMarkdownFormatter(commitUrlTemplate, cfg)
	require.NoError(t, err)

	anomalygroup := &v1.AnomalyGroup{
		GroupId:       "group_id",
		AnomalyIds:    []string{"a_id_1", "a_id_2", "a_id_3"},
		BenchmarkName: "Benchmark_Test",
	}

	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   234,
			Paramset: map[string]string{
				"bot":         "b",
				"benchmark":   "bc",
				"story":       "s",
				"measurement": "m",
			},
			MedianBefore: 2.0,
			MedianAfter:  3.0,
		},
		{
			StartCommit: 1234,
			EndCommit:   2345,
			Paramset: map[string]string{
				"bot":         "bb",
				"benchmark":   "bbcc",
				"story":       "ss",
				"measurement": "mm",
			},
			MedianBefore: 0.4444,
			MedianAfter:  0.3333,
		},
	}

	subject, body, err := f.GetReportSubjectAndBody(context.Background(), anomalygroup, subscription, anomalies)
	assert.Nil(t, err)
	assert.True(t, strings.Contains(string(body), "test-url/u/?anomalyGroupID=group_id"), body)
	assert.Equal(t, subject, "[Subscription_Test]: [3] regressions in Benchmark_Test", subject)
	assert.True(t, strings.Contains(string(body), "Top 2 anomalies in this group:"), body)
	assert.True(t, strings.Contains(string(body), "Bot: b, Benchmark: bc, Measurement: m, Story: s,"), body)
	assert.True(t, strings.Contains(string(body), "-25.00%"), body) // 0.4444 -> 0.3333
}

func TestFormatting_Report_WithConfig(t *testing.T) {
	commitUrlTemplate := "https://skia.googlesource.com/skia/+log/%s"
	cfg := &config.InstanceConfig{
		IssueTrackerConfig: config.IssueTrackerConfig{
			AnomalyReportSubject: "Simple title: [{{ .Subscription.Name }}]",
			AnomalyReportBody: []string{
				"Simple body: line 1",
				"line 2: {{ .AnomalyGroup.BenchmarkName }}",
			},
		},
	}
	subscription := &sub_pb.Subscription{
		Name: "Subscription_Test",
	}
	f, err := NewMarkdownFormatter(commitUrlTemplate, cfg)
	require.NoError(t, err)

	anomalygroup := &v1.AnomalyGroup{
		GroupId:       "group_id",
		AnomalyIds:    []string{"a_id_1", "a_id_2", "a_id_3"},
		BenchmarkName: "Benchmark_Test",
	}

	anomalies := []*pb.Anomaly{
		{
			StartCommit: 123,
			EndCommit:   234,
			Paramset: map[string]string{
				"bot":         "b",
				"benchmark":   "bc",
				"story":       "s",
				"measurement": "m",
			},
		},
		{
			StartCommit: 1234,
			EndCommit:   2345,
			Paramset: map[string]string{
				"bot":         "bb",
				"benchmark":   "bbcc",
				"story":       "ss",
				"measurement": "mm",
			},
		},
	}

	subject, body, err := f.GetReportSubjectAndBody(context.Background(), anomalygroup, subscription, anomalies)
	fmt.Println("====", body)
	assert.Nil(t, err)
	assert.Equal(t, subject, "Simple title: [Subscription_Test]")
	assert.True(t, strings.Contains(string(body), "Simple body: line 1"))
	assert.True(t, strings.Contains(string(body), "line 2: Benchmark_Test"))
}
