package issues

import (
	"fmt"
	"testing"

	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/testutils"
)

func TestCreateBadBugURL(t *testing.T) {
	testutils.SmallTest(t)
	// No http calls need to be mocked up, as none should be used.
	urlMock := mockhttpclient.NewURLMock()
	im := NewManager(urlMock.Client())
	p := IssueReportingPackage{
		FuzzName:       "1234567890abcdef",
		CommitRevision: "fedcba9876543210",
		Category:       "api_pathop",
	}
	url, err := im.CreateBadBugURL(p)
	if err != nil {
		t.Errorf("Should not have returned error: %s", err)
	}
	expectedURL := `https://bugs.chromium.org/p/skia/issues/entry?cc=kjlubick%40google.com&comment=%23+Description+here+about+fuzz+found+in+API+-+PathOp%0A%0A%0ATo+replicate%2C+build+target+%22fuzz%22+at+the+specified+commit+and+run%3A%0Aout%2FRelease%2Ffuzz+--type+api+--name+Pathop+--bytes+~%2FDownloads%2Fapi-Pathop-1234567890abcdef%0A%0AThe+problem+may+only+be+revealed+by+an+ASAN+build%2C+in+which+case+you+would+need+to+run%3A%0Agn+gen+out%2FASAN+--args%3D%27cc%3D%22%2Fusr%2Fbin%2Fclang%22+cxx%3D%22%2Fusr%2Fbin%2Fclang%2B%2B%22+sanitize%3D%22ASAN%22%27%0Aor%3A%0Agn+gen+out%2FASAN+--args%3D%27cc%3D%22%2Fusr%2Fbin%2Fclang%22+cxx%3D%22%2Fusr%2Fbin%2Fclang%2B%2B%22+sanitize%3D%22ASAN%22+is_debug%3Dfalse%27%0A%0Aprior+to+building.%0A%0A%23+tracking+metadata+below%3A%0Afuzz_category%3A+api_pathop%0Afuzz_commit%3A+fedcba9876543210%0Arelated_fuzz%3A+https%3A%2F%2Ffuzzer.skia.org%2Fcategory%2Fapi_pathop%2Fname%2F1234567890abcdef%0Afuzz_download%3A+https%3A%2F%2Ffuzzer.skia.org%2Ffuzz%2F1234567890abcdef%0A&labels=FromSkiaFuzzer%2CRestrict-View-Google%2CType-Defect%2CPriority-Medium&owner=caryclark%40google.com&status=New&summary=New+crash+found+in+API+-+PathOp+by+fuzzer`
	if url != expectedURL {
		t.Errorf("URL does not match.  Expected: %s\n\nWas: %s\n", expectedURL, url)
	}

}

func TestCreateBadBugIssue(t *testing.T) {
	testutils.SmallTest(t)
	urlMock := mockhttpclient.NewURLMock()
	im := NewManager(urlMock.Client())
	p := IssueReportingPackage{
		FuzzName:       "1234567890abcdef",
		CommitRevision: "fedcba9876543210",
		Category:       "api_parse_path",
	}

	urlMock.MockOnce(fmt.Sprintf(issues.MONORAIL_BASE_URL_TMPL, issues.PROJECT_SKIA), mockhttpclient.MockPostDialogue("application/json", expectedIssueRequest, []byte(exampleMonorailResponse)))

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
