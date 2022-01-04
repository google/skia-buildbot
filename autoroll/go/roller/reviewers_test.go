package roller

import (
	"testing"

	"github.com/stretchr/testify/require"
	metrics2_testutils "go.skia.org/infra/go/metrics2/testutils"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils/unittest"
)

const testRollerName = "fake-roller"

func checkSuccessMetric(t *testing.T, source string, expectSuccess bool) {
	tags := map[string]string{
		"roller":          testRollerName,
		"reviewer_source": source,
	}
	expect := "0"
	if expectSuccess {
		expect = "1"
	}
	require.Equal(t, expect, metrics2_testutils.GetRecordedMetric(t, measurementGetReviewersSuccess, tags), source)
}

func checkSourceFailed(t *testing.T, source string) {
	checkSuccessMetric(t, source, false)
}

func checkSourceSucceeded(t *testing.T, source string) {
	checkSuccessMetric(t, source, true)
}

func checkNonEmptyMetric(t *testing.T, expectNonEmpty bool) {
	tags := map[string]string{
		"roller": testRollerName,
	}
	expect := "0"
	if expectNonEmpty {
		expect = "1"
	}
	require.Equal(t, expect, metrics2_testutils.GetRecordedMetric(t, measurementGetReviewersNonEmpty, tags))
}

func checkReviewersEmpty(t *testing.T) {
	checkNonEmptyMetric(t, false)
}

func checkReviewersNonEmpty(t *testing.T) {
	checkNonEmptyMetric(t, true)
}

func TestGetReviewers(t *testing.T) {
	unittest.SmallTest(t)

	urlMock := mockhttpclient.NewURLMock()
	urlMock.MockOnce("https://reviewers.com/fake", mockhttpclient.MockGetDialogue([]byte(`{"emails": ["you@google.com", "me@google.com"]}`)))
	reviewersSources := []string{
		"https://reviewers.com/fake",
		"explicit-reviewer@google.com",
		"https://unknown-url.com",
	}
	backupReviewers := []string{"backup-reviewer@google.com"}

	gotReviewers := GetReviewers(urlMock.Client(), testRollerName, reviewersSources, backupReviewers)
	require.Equal(t, []string{"explicit-reviewer@google.com", "me@google.com", "you@google.com"}, gotReviewers)
	checkSourceSucceeded(t, "https://reviewers.com/fake")
	checkSourceSucceeded(t, "explicit-reviewer@google.com")
	checkSourceFailed(t, "https://unknown-url.com")
	checkReviewersNonEmpty(t)
}

func TestGetReviewers_Backup(t *testing.T) {
	unittest.SmallTest(t)

	backupReviewers := []string{"backup-reviewer@google.com"}
	gotReviewers := GetReviewers(nil, testRollerName, []string{"bad-source"}, backupReviewers)
	require.Equal(t, backupReviewers, gotReviewers)
	checkSourceFailed(t, "bad-source")
	checkReviewersEmpty(t)
}
