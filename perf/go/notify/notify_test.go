package notify

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/email"
	email_mocks "go.skia.org/infra/go/email/mocks"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/notify/common"
	"go.skia.org/infra/perf/go/notify/mocks"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/ui/frame"
)

const (
	newHTMLMessage = "<b>Alert</b><br><br>\n<p>\n\tA Perf Regression (High) has been found at:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\">https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n  For:\n</p>\n<p style=\"padding: 1em;\">\n  <a href=\"https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664\">https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tWith 10 matching traces.\n</p>\n<p>\n   And direction High.\n</p>\n<p>\n\tFrom Alert <a href=\"https://perf.skia.org/a/?123\">MyAlert</a>\n</p>\n"
	newHTMLSubject = "MyAlert - Regression found for d261e10 -  2y 40w - An example commit use for testing."

	missingHTMLMessage = "<b>Alert</b><br><br>\n<p>\n\tA Perf Regression (High) can no longer be found at:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\">https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tFor:\n</p>\n<p style=\"padding: 1em;\">\n\t<a href=\"https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664\">https://skia.googlesource.com/skia/&#43;show/d261e1075a93677442fdf7fe72aba7e583863664</a>\n</p>\n<p>\n\tWith 10 matching traces.\n</p>\n<p>\n\tAnd direction High.\n</p>\n<p>\n\tFrom Alert <a href=\"https://perf.skia.org/a/?123\">MyAlert</a>\n</p>\n"
	missingHTMLSubject = "MyAlert - Regression no longer found for d261e10 -  2y 40w - An example commit use for testing."

	newMarkdownMessage = "A Perf Regression (High) has been found at:\n\n  https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\n\nFor:\n\n  Commit https://skia.googlesource.com/skia/+show/d261e1075a93677442fdf7fe72aba7e583863664\n\nWith:\n\n  - 10 matching traces.\n  - Direction High.\n\nFrom Alert [MyAlert](https://perf.skia.org/a/?123)\n"
	newMarkdownSubject = "MyAlert - Regression found for An example commit use for testing."

	missingMarkdownMessage = "The Perf Regression can no longer be detected. This issue is being automatically closed.\n"
	missingMarkdownSubject = "MyAlert - Regression no longer found for An example commit use for testing."

	newMarkdownMessageWithCommitRangeURLTemplate = "A Perf Regression (High) has been found at:\n\n  https://perf.skia.org/g/t/d261e1075a93677442fdf7fe72aba7e583863664\n\nFor:\n\n  Commit https://example.com/fb49909acafba5e031b90a265a6ce059cda85019/d261e1075a93677442fdf7fe72aba7e583863664/\n\nWith:\n\n  - 10 matching traces.\n  - Direction High.\n\nFrom Alert [MyAlert](https://perf.skia.org/a/?123)\n"
)

const (
	mockThreadingID = "123456789"
	instanceURL     = "https://perf.skia.org"
)

var (
	alertForTest = &alerts.Alert{
		IDAsString:  "123",
		Alert:       "someone@example.org, someother@example.com ",
		DisplayName: "MyAlert",
	}

	errMock = errors.New("my mock error")

	commitForTestingBuildID = provider.Commit{
		Subject: "https://android-build.googleplex.com/builds/jump-to-build/10768667 ",
	}

	previousCommitForTestingBuildID = provider.Commit{
		Subject: "https://android-build.googleplex.com/builds/jump-to-build/10768666 ",
	}

	cl = &clustering2.ClusterSummary{
		Num: 10,
		StepFit: &stepfit.StepFit{
			Status: stepfit.HIGH,
		},
		StepPoint: &dataframe.ColumnHeader{
			Offset: 2,
		},
	}

	frameResponse = &frame.FrameResponse{
		DataFrame: &dataframe.DataFrame{
			Header: []*dataframe.ColumnHeader{
				{Offset: 1, Timestamp: 1687824470},
				{Offset: 2, Timestamp: 1498176000},
			},
			ParamSet: paramtools.ReadOnlyParamSet{
				"device_name": []string{"sailfish", "sargo", "wembley"},
			},
		},
	}
)

func TestExampleSendWithHTMLFormatter_HappyPath(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newHTMLMessage, newHTMLSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingHTMLMessage, missingHTMLSubject).Return(nil)
	tr.On("UpdateRegressionNotification", testutils.AnyContext, alertForTest, newHTMLMessage, mockThreadingID).Return(nil)

	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newHTMLMessage,
		Subject: newHTMLSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingHTMLMessage,
		Subject: missingHTMLSubject,
	}, nil)
	n := newNotifier(ndp, NewHTMLFormatter(""), tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestExampleSendWithMarkdownFormatter_HappyPath(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newMarkdownMessage, newMarkdownSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingMarkdownMessage, missingMarkdownSubject).Return(nil)
	tr.On("UpdateRegressionNotification", testutils.AnyContext, alertForTest, newMarkdownMessage, mockThreadingID).Return(nil)

	f, err := NewMarkdownFormatter("", &config.NotifyConfig{})
	require.NoError(t, err)

	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newMarkdownMessage,
		Subject: newMarkdownSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingMarkdownMessage,
		Subject: missingMarkdownSubject,
	}, nil)
	n := newNotifier(ndp, f, tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err = n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestExampleSendWithMarkdownFormatterWithCommitRangeURLTemplate_HappyPath(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newMarkdownMessageWithCommitRangeURLTemplate, newMarkdownSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingMarkdownMessage, missingMarkdownSubject).Return(nil)
	tr.On("UpdateRegressionNotification", testutils.AnyContext, alertForTest, newMarkdownMessageWithCommitRangeURLTemplate, mockThreadingID).Return(nil)

	f, err := NewMarkdownFormatter("https://example.com/{begin}/{end}/", &config.NotifyConfig{})
	require.NoError(t, err)
	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newMarkdownMessageWithCommitRangeURLTemplate,
		Subject: newMarkdownSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingMarkdownMessage,
		Subject: missingMarkdownSubject,
	}, nil)
	n := newNotifier(ndp, f, tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err = n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestExampleSendWithMarkdownFormatterWithCommitRangeURLTemplateAndCustomizedNotifierFormats_HappyPath(t *testing.T) {
	tr := mocks.NewTransport(t)
	newMessage := "body MyAlert - https://example.com/fb49909acafba5e031b90a265a6ce059cda85019/d261e1075a93677442fdf7fe72aba7e583863664/"
	newSubject := "subject fb49909acafba5e031b90a265a6ce059cda85019"
	tr.On(
		"SendNewRegression",
		testutils.AnyContext,
		alertForTest,
		newMessage,
		newSubject,
	).Return(mockThreadingID, nil)
	missingMessage := "missing-body MyAlert - https://example.com/fb49909acafba5e031b90a265a6ce059cda85019/d261e1075a93677442fdf7fe72aba7e583863664/"
	missingSubject := "missing-subject fb49909acafba5e031b90a265a6ce059cda85019"
	tr.On(
		"SendRegressionMissing",
		testutils.AnyContext,
		mockThreadingID,
		alertForTest,
		missingMessage,
		missingSubject,
	).Return(nil)
	tr.On(
		"UpdateRegressionNotification",
		testutils.AnyContext,
		alertForTest,
		newMessage,
		mockThreadingID,
	).Return(nil)

	f, err := NewMarkdownFormatter("https://example.com/{begin}/{end}/", &config.NotifyConfig{
		Subject:        "subject {{ .PreviousCommit.GitHash }}",
		Body:           []string{"body {{ .Alert.DisplayName }} - {{ .CommitURL }}"},
		MissingSubject: "missing-subject {{ .PreviousCommit.GitHash }}",
		MissingBody:    []string{"missing-body {{ .Alert.DisplayName }} - {{ .CommitURL }}"},
	})
	require.NoError(t, err)
	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newMessage,
		Subject: newSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingMessage,
		Subject: missingSubject,
	}, nil)
	n := newNotifier(ndp, f, tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err = n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestExampleSendWithHTMLFormatter_SendRegressionMissingReturnsError_ReturnsError(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newHTMLMessage, newHTMLSubject).Return(mockThreadingID, nil)
	tr.On("SendRegressionMissing", testutils.AnyContext, mockThreadingID, alertForTest, missingHTMLMessage, missingHTMLSubject).Return(errMock)

	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newHTMLMessage,
		Subject: newHTMLSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingHTMLMessage,
		Subject: missingHTMLSubject,
	}, nil)
	n := newNotifier(ndp, NewHTMLFormatter(""), tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.ErrorIs(t, err, errMock)
	require.Contains(t, err.Error(), "sending regression missing message")
}

func TestExampleSendWithHTMLFormatter_SendNewRegressionReturnsError_ReturnsError(t *testing.T) {
	tr := mocks.NewTransport(t)
	tr.On("SendNewRegression", testutils.AnyContext, alertForTest, newHTMLMessage, newHTMLSubject).Return("", errMock)

	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newHTMLMessage,
		Subject: newHTMLSubject,
	}, nil)
	n := newNotifier(ndp, NewHTMLFormatter(""), tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.ErrorIs(t, err, errMock)
	require.Contains(t, err.Error(), "sending new regression message")
}

func TestExampleSendWithHTMLFormatterAndEMailTransport_HappyPath(t *testing.T) {
	const expectedMessageID = "<the-actual-message-id>"

	emailClient := &email_mocks.Client{}
	tr := EmailTransport{
		client: emailClient,
	}
	subjects := []string{newHTMLSubject, missingHTMLSubject}
	subjectIndex := 0
	emailClient.On("SendMail", testutils.AnyContext, mock.MatchedBy(func(req *email.SendMailRequest) bool {
		require.Contains(t, req.HtmlBody, "<b>Alert</b>")
		require.Contains(t, req.Subject, subjects[subjectIndex])
		subjectIndex++
		return true
	})).Return(&email.SendMailResponse{
		MessageId: mockThreadingID,
	}, nil)
	ndp := mocks.NewNotificationDataProvider(t)
	ndp.On("GetNotificationDataRegressionFound", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    newHTMLMessage,
		Subject: newHTMLSubject,
	}, nil)
	ndp.On("GetNotificationDataRegressionMissing", testutils.AnyContext, mock.Anything).Return(&common.NotificationData{
		Body:    missingHTMLMessage,
		Subject: missingHTMLSubject,
	}, nil)
	n := newNotifier(ndp, NewHTMLFormatter(""), tr, instanceURL, nil, nil)
	ctx := context.WithValue(context.Background(), now.ContextKey, time.Date(2020, 04, 01, 0, 0, 0, 0, time.UTC))
	err := n.ExampleSend(ctx, alertForTest)
	require.NoError(t, err)
}

func TestMarkdownFormatter_CallsBuildIDFromSubjectButSubjectDoesntContainLink_ReturnsEmptyString(t *testing.T) {
	f, err := NewMarkdownFormatter("", &config.NotifyConfig{
		Subject: "From {{ buildIDFromSubject .PreviousCommit.Subject}} To {{ buildIDFromSubject .Commit.Subject}}",
	})
	require.NoError(t, err)

	_, subject, err := f.FormatNewRegression(context.Background(), commit, previousCommit, alertForTest, cl, "", frameResponse)
	require.NoError(t, err)
	require.Equal(t, "From  To ", subject)
}

func TestMarkdownFormatter_CallsBuildIDFromSubject_Success(t *testing.T) {
	f, err := NewMarkdownFormatter("", &config.NotifyConfig{
		Subject: "From {{ buildIDFromSubject .PreviousCommit.Subject}} To {{ buildIDFromSubject .Commit.Subject}}",
	})
	require.NoError(t, err)

	_, subject, err := f.FormatNewRegression(context.Background(), commitForTestingBuildID, previousCommitForTestingBuildID, alertForTest, cl, "", frameResponse)
	require.NoError(t, err)
	require.Equal(t, "From 10768666 To 10768667", subject)
}

func TestMarkdownFormatter_ForLoopOnParamSet_Success(t *testing.T) {
	f, err := NewMarkdownFormatter("", &config.NotifyConfig{
		Subject: "devices: {{ range index .ParamSet \"device_name\" }} {{ . }} | {{ end }}",
	})
	require.NoError(t, err)

	_, subject, err := f.FormatNewRegression(context.Background(), commitForTestingBuildID, previousCommitForTestingBuildID, alertForTest, cl, "", frameResponse)
	require.NoError(t, err)
	require.Equal(t, "devices:  sailfish |  sargo |  wembley | ", subject)
}
