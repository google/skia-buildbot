package issues

import (
	"testing"

	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestCreateBadBugIssue(t *testing.T) {
	testutils.SmallTest(t)
	urlMock := mockhttpclient.NewURLMock()
	im := NewManager(urlMock.Client())
	p := IssueReportingPackage{
		FuzzName:       "1234567890abcdef",
		CommitRevision: "fedcba9876543210",
		Category:       "api_parse_path",
	}

	urlMock.MockOnce(issues.MONORAIL_BASE_URL, mockhttpclient.MockPostDialogue("application/json", expectedIssueRequest, []byte(exampleMonorailResponse)))

	err := im.CreateBadBugIssue(p, "Mock fuzzer found a problem")
	if err != nil {
		t.Errorf("Should not have returned error: %s", err)
	}
}

var expectedIssueRequest = []byte(`{"status":"New","owner":{"name":"caryclark@google.com","htmlLink":"","kind":""},"cc":[{"name":"kjlubick@google.com","htmlLink":"","kind":""}],"labels":["FromSkiaFuzzer","Restrict-View-Google","Type-Defect","Priority-Medium"],"summary":"New crash found in API - ParsePath by fuzzer","description":"# Description here about fuzz found in API - ParsePath\nMock fuzzer found a problem\n\nTo replicate, build target \"fuzz\" at the specified commit and run:\nout/Release/fuzz --type api --name ParsePath --bytes ~/Downloads/api-ParsePath-1234567890abcdef\n\nThe problem may only be revealed by an ASAN build, in which case you would need to run:\ngn gen out/ASAN --args='cc=\"/usr/bin/clang\" cxx=\"/usr/bin/clang++\" sanitize=\"ASAN\"'\nor:\ngn gen out/ASAN --args='cc=\"/usr/bin/clang\" cxx=\"/usr/bin/clang++\" sanitize=\"ASAN\" is_debug=false'\n\nprior to building.\n\n# tracking metadata below:\nfuzz_category: api_parse_path\nfuzz_commit: fedcba9876543210\nrelated_fuzz: https://fuzzer.skia.org/category/api_parse_path/name/1234567890abcdef\nfuzz_download: https://fuzzer.skia.org/fuzz/1234567890abcdef\n"}
`)

var exampleMonorailResponse = `{
 "status": "New",
 "updated": "2016-05-09T14:37:43",
 "canEdit": true,
 "author": {
  "kind": "monorail#issuePerson",
  "htmlLink": "https://bugs.chromium.org/u/redactedNumbers",
  "name": "service-account-@redacted.com"
 },
 "projectId": "skia",
 "labels": [
  "Type-Defect",
  "Priority-Medium",
  "Restrict-View-Google"
 ],
 "kind": "monorail#issue",
 "canComment": true,
 "state": "open",
 "stars": 0,
 "published": "2016-05-09T14:37:43",
 "title": "Another test bug",
 "starred": false,
 "summary": "Another test bug",
 "id": 5268,
 "etag": "\"FCnnF6QwisNABmHbGpwISZgQNXk/D3OWSf3kqXOPmm4kavoM01N4mLc\""
}`
