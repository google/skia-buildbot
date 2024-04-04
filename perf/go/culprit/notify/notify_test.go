package notify

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	ft_mocks "go.skia.org/infra/perf/go/culprit/formatter/mocks"
	pb "go.skia.org/infra/perf/go/culprit/proto/v1"
	tr_mocks "go.skia.org/infra/perf/go/culprit/transport/mocks"
	sub_pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

func TestNotifyCulpritFound_HappyPath(t *testing.T) {
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
	formatter := ft_mocks.NewFormatter(t)
	formatter.On("GetSubjectAndBody", mock.Anything, culprit, subscription).Return("subject", "body", nil)
	transport := tr_mocks.NewTransport(t)
	expectedBugId := "bug123"
	transport.On("SendNewCulprit", mock.Anything, mock.Anything, "subject", "body").Return(expectedBugId, nil)

	n := &DefaultCulpritNotifier{
		formatter: formatter,
		transport: transport,
	}

	actualBugId, err := n.NotifyCulpritFound(context.Background(), culprit, subscription)

	assert.Nil(t, err)
	assert.Equal(t, expectedBugId, actualBugId)

}
