package formatter

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/config"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func TestFormatting_HappyPath(t *testing.T) {
	commitUrlTemplate := "https://skia.googlesource.com/skia/+log/%s"
	cfg := &config.CulpritNotifyConfig{}
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
	subject, body, err := f.GetSubjectAndBody(context.Background(), culprit, subscription)
	fmt.Println(subject)
	fmt.Println(body)
	assert.True(t, strings.Contains(string(subject), subscription.Name))
	assert.True(t, strings.Contains(string(body), fmt.Sprintf(commitUrlTemplate, culprit.Commit.Revision)))
	assert.Nil(t, err)
}
