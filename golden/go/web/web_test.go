package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/clstore"
	mock_clstore "go.skia.org/infra/golden/go/clstore/mocks"
	"go.skia.org/infra/golden/go/code_review"
	mock_crs "go.skia.org/infra/golden/go/code_review/mocks"
	"go.skia.org/infra/golden/go/image/text"
	"go.skia.org/infra/golden/go/mocks"
	"go.skia.org/infra/golden/go/search2"
	mock_search2 "go.skia.org/infra/golden/go/search2/mocks"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/sqltest"
	one_by_five "go.skia.org/infra/golden/go/testutils/data_one_by_five"
	"go.skia.org/infra/golden/go/types"
	"go.skia.org/infra/golden/go/web/frontend"
)

func TestStubbedAuthAs_OverridesLoginLogicWithHardCodedEmail(t *testing.T) {
	unittest.SmallTest(t)
	r := httptest.NewRequest(http.MethodGet, "/does/not/matter", nil)
	wh := Handlers{}
	assert.Equal(t, "", wh.loggedInAs(r))

	const fakeUser = "user@example.com"
	wh.testingAuthAs = fakeUser
	assert.Equal(t, fakeUser, wh.loggedInAs(r))
}

// TestNewHandlers_BaselineSubset_HasAllPieces_Success makes sure we can create a web.Handlers
// using the BaselineSubset of inputs.
func TestNewHandlers_BaselineSubset_HasAllPieces_Success(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{
		GCSClient: &mocks.GCSClient{},
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:     "whatever",
				Store:  &mock_clstore.Store{},
				Client: &mock_crs.Client{},
			},
		},
		DB: &pgxpool.Pool{},
	}
	_, err := NewHandlers(hc, BaselineSubset)
	require.NoError(t, err)
}

// TestNewHandlers_BaselineSubset_MissingPieces_Failure makes sure that if we omit values from
// HandlersConfig, NewHandlers returns an error.
func TestNewHandlers_BaselineSubset_MissingPieces_Failure(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{}
	_, err := NewHandlers(hc, BaselineSubset)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	hc = HandlersConfig{
		GCSClient: &mocks.GCSClient{},
	}
	_, err = NewHandlers(hc, BaselineSubset)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be empty")
}

// TestNewHandlers_FullFront_EndMissingPieces_Failure makes sure that if we omit values from
// HandlersConfig, NewHandlers returns an error.
// TODO(kjlubick) Add a case for FullFrontEnd with all pieces when we have mocks for all
//   remaining services.
func TestNewHandlers_FullFrontEnd_MissingPieces_Failure(t *testing.T) {
	unittest.SmallTest(t)

	hc := HandlersConfig{}
	_, err := NewHandlers(hc, FullFrontEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")

	hc = HandlersConfig{
		GCSClient: &mocks.GCSClient{},
		ReviewSystems: []clstore.ReviewSystem{
			{
				ID:     "whatever",
				Store:  &mock_clstore.Store{},
				Client: &mock_crs.Client{},
			},
		},
	}
	_, err = NewHandlers(hc, FullFrontEnd)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be nil")
}

// TestGetIngestedChangelists_AllChangelists_SunnyDay_Success tests the core functionality of
// listing all Changelists that have Gold results.
func TestGetIngestedChangelists_AllChangelists_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	const offset = 0
	const size = 50

	mcls.On("GetChangelists", testutils.AnyContext, clstore.SearchOptions{
		StartIdx: offset,
		Limit:    size,
	}).Return(makeCodeReviewCLs(), len(makeCodeReviewCLs()), nil)

	wh := Handlers{
		anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
		HandlersConfig: HandlersConfig{
			ReviewSystems: []clstore.ReviewSystem{
				{
					ID:          "gerrit",
					Store:       mcls,
					URLTemplate: "example.com/cl/%s#templates",
				},
			},
		},
	}

	cls := makeWebCLs()

	expectedResponse := frontend.ChangelistsResponse{
		Changelists: cls,
		ResponsePagination: httputils.ResponsePagination{
			Offset: offset,
			Size:   size,
			Total:  len(cls),
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/json/v1/changelist?size=50", nil)
	wh.ChangelistsHandler(w, r)
	b, err := json.Marshal(expectedResponse)
	require.NoError(t, err)
	assertJSONResponseWas(t, http.StatusOK, string(b), w)
}

// TestGetIngestedChangelists_ActiveChangelists_SunnyDay_Success makes sure that we properly get
// only active Changelists, that is, Changelists which are open.
func TestGetIngestedChangelists_ActiveChangelists_SunnyDay_Success(t *testing.T) {
	unittest.SmallTest(t)

	mcls := &mock_clstore.Store{}
	defer mcls.AssertExpectations(t)

	const offset = 20
	const size = 30

	mcls.On("GetChangelists", testutils.AnyContext, clstore.SearchOptions{
		StartIdx:    offset,
		Limit:       size,
		OpenCLsOnly: true,
	}).Return(makeCodeReviewCLs(), 3, nil)

	wh := Handlers{
		anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
		HandlersConfig: HandlersConfig{
			ReviewSystems: []clstore.ReviewSystem{
				{
					ID:          "gerrit",
					Store:       mcls,
					URLTemplate: "example.com/cl/%s#templates",
				},
			},
		},
	}

	cls := makeWebCLs()

	expectedResponse := frontend.ChangelistsResponse{
		Changelists: cls,
		ResponsePagination: httputils.ResponsePagination{
			Offset: offset,
			Size:   size,
			Total:  len(cls),
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/json/v1/changelist?offset=20&size=30&active=true", nil)
	wh.ChangelistsHandler(w, r)
	b, err := json.Marshal(expectedResponse)
	require.NoError(t, err)
	assertJSONResponseWas(t, http.StatusOK, string(b), w)
}

func makeCodeReviewCLs() []code_review.Changelist {
	return []code_review.Changelist{
		{
			SystemID: "1002",
			Owner:    "other@example.com",
			Status:   code_review.Open,
			Subject:  "new feature",
			Updated:  time.Date(2019, time.August, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   code_review.Landed,
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
		},
		{
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   code_review.Abandoned,
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
		},
	}
}

func makeWebCLs() []frontend.Changelist {
	return []frontend.Changelist{
		{
			System:   "gerrit",
			SystemID: "1002",
			Owner:    "other@example.com",
			Status:   "Open",
			Subject:  "new feature",
			Updated:  time.Date(2019, time.August, 27, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1002#templates",
		},
		{
			System:   "gerrit",
			SystemID: "1001",
			Owner:    "test@example.com",
			Status:   "Landed",
			Subject:  "land gold",
			Updated:  time.Date(2019, time.August, 26, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1001#templates",
		},
		{
			System:   "gerrit",
			SystemID: "1000",
			Owner:    "test@example.com",
			Status:   "Abandoned",
			Subject:  "gold experiment",
			Updated:  time.Date(2019, time.August, 25, 0, 0, 0, 0, time.UTC),
			URL:      "example.com/cl/1000#templates",
		},
	}
}

// TestHandlersThatRequireLogin_NotLoggedIn_UnauthorizedError tests a list of handlers to make sure
// they return an Unauthorized status if attempted to be used without being logged in.
func TestHandlersThatRequireLogin_NotLoggedIn_UnauthorizedError(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{}

	test := func(name string, endpoint http.HandlerFunc) {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("does not matter"))
			endpoint(w, r)

			resp := w.Result()
			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		})
	}
	test("add", wh.AddIgnoreRule2)
	test("update", wh.UpdateIgnoreRule2)
	test("delete", wh.DeleteIgnoreRule2)
	// TODO(kjlubick): check all handlers that need login, not just Ignores*
}

// TestHandlersWhichTakeJSON_BadInput_BadRequestError tests a list of handlers which take JSON as an
// input and make sure they all return a BadRequest response when given bad input.
func TestHandlersWhichTakeJSON_BadInput_BadRequestError(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		testingAuthAs: "test@google.com",
	}

	test := func(name string, endpoint http.HandlerFunc) {
		t.Run(name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader("invalid JSON"))
			endpoint(w, r)

			resp := w.Result()
			assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		})
	}
	test("add", wh.AddIgnoreRule2)
	test("update", wh.UpdateIgnoreRule2)
	// TODO(kjlubick): check all handlers that process JSON
}

// TestGetValidatedIgnoreRule_InvalidInput_Error tests several exceptional cases where an invalid
// rule is given to the handler.
func TestGetValidatedIgnoreRule_InvalidInput_Error(t *testing.T) {
	unittest.SmallTest(t)

	test := func(name, errorFragment, jsonInput string) {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, requestURL, strings.NewReader(jsonInput))
			_, _, err := getValidatedIgnoreRule(r)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), errorFragment)
		})
	}

	test("invalid JSON", "request JSON", "This should not be valid JSON")
	// There's an instagram joke here... #nofilter
	test("no filter", "supply a filter", `{"duration": "1w", "filter": "", "note": "skbug:9744"}`)
	test("no duration", "invalid duration", `{"duration": "", "filter": "a=b", "note": "skbug:9744"}`)
	test("invalid duration", "invalid duration", `{"duration": "bad", "filter": "a=b", "note": "skbug:9744"}`)
	test("filter too long", "Filter must be", string(makeJSONWithLongFilter(t)))
	test("note too long", "Note must be", string(makeJSONWithLongNote(t)))
}

// makeJSONWithLongFilter returns a []byte that is the encoded JSON of an otherwise valid
// IgnoreRuleBody, except it has a Filter which exceeds 10 KB.
func makeJSONWithLongFilter(t *testing.T) []byte {
	superLongFilter := frontend.IgnoreRuleBody{
		Duration: "1w",
		Filter:   strings.Repeat("a=b&", 10000),
		Note:     "really long filter",
	}
	superLongFilterBytes, err := json.Marshal(superLongFilter)
	require.NoError(t, err)
	return superLongFilterBytes
}

// makeJSONWithLongNote returns a []byte that is the encoded JSON of an otherwise valid
// IgnoreRuleBody, except it has a Note which exceeds 1 KB.
func makeJSONWithLongNote(t *testing.T) []byte {
	superLongFilter := frontend.IgnoreRuleBody{
		Duration: "1w",
		Filter:   "a=b",
		Note:     strings.Repeat("really long note ", 1000),
	}
	superLongFilterBytes, err := json.Marshal(superLongFilter)
	require.NoError(t, err)
	return superLongFilterBytes
}

// TestBaselineHandlerV2_Success tests that the handler correctly calls the BaselineFetcher when no
// GET parameters are set.
func TestBaselineHandlerV2_Success(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

// TestBaselineHandlerV2_IssueSet_Success tests that the handler correctly calls the BaselineFetcher
// when the "issue" GET parameter is set.
func TestBaselineHandlerV2_IssueSet_Success(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

// TestBaselineHandlerV2_IssueSet_Success tests that the handler correctly calls the BaselineFetcher
// when the "issue" and "issueOnly" GET parameters are set.
func TestBaselineHandlerV2_IssueSet_IssueOnly_Success(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

// TestWhoami_NotLoggedIn_Success tests that /json/whoami returns the expected empty response when
// no user is logged in.
func TestWhoami_NotLoggedIn_Success(t *testing.T) {
	unittest.SmallTest(t)
	wh := Handlers{
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.Whoami(w, r)
	assertJSONResponseWas(t, http.StatusOK, `{"whoami":""}`, w)
}

// TestWhoami_LoggedIn_Success tests that /json/whoami returns the email of the user that is
// currently logged in.
func TestWhoami_LoggedIn_Success(t *testing.T) {
	unittest.SmallTest(t)
	wh := Handlers{
		anonymousCheapQuota: rate.NewLimiter(rate.Inf, 1),
		testingAuthAs:       "test@example.com",
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	wh.Whoami(w, r)
	assertJSONResponseWas(t, http.StatusOK, `{"whoami":"test@example.com"}`, w)
}

// TestLatestPositiveDigest_Success tests that /json/latestpositivedigest/{traceId} returns the
// most recent positive digest in the expected format.
//
// Note: We don't test the cases when the tile has no positive digests, or when the trace isn't
// found, because it both cases SearchIndexer method MostRecentPositiveDigest() will just return
// types.MissingDigest and a nil error.
func TestLatestPositiveDigest_Success(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

func TestChangelistSearchRedirect_CLHasUntriagedDigests_Success(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

func TestChangelistSearchRedirect_CLDoesNotExist_404Error(t *testing.T) {
	unittest.SmallTest(t)

	// TODO(kjlubick)
}

func TestImageHandler_SingleKnownImage_CorrectBytesReturned(t *testing.T) {
	unittest.SmallTest(t)

	mgc := &mocks.GCSClient{}
	mgc.On("GetImage", testutils.AnyContext, types.Digest("0123456789abcdef0123456789abcdef")).Return([]byte("some png bytes"), nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			GCSClient: mgc,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/images/0123456789abcdef0123456789abcdef.png", nil)
	wh.ImageHandler(w, r)
	assertImageResponseWas(t, []byte("some png bytes"), w)
}

func TestImageHandler_SingleUnknownImage_404Returned(t *testing.T) {
	unittest.SmallTest(t)

	mgc := &mocks.GCSClient{}
	mgc.On("GetImage", testutils.AnyContext, mock.Anything).Return(nil, errors.New("unknown"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			GCSClient: mgc,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/images/0123456789abcdef0123456789abcdef.png", nil)
	wh.ImageHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestImageHandler_TwoKnownImages_DiffReturned(t *testing.T) {
	unittest.SmallTest(t)

	image1 := loadAsPNGBytes(t, one_by_five.ImageOne)
	image2 := loadAsPNGBytes(t, one_by_five.ImageTwo)
	mgc := &mocks.GCSClient{}
	// These digests are arbitrary - they do not match the provided images.
	mgc.On("GetImage", testutils.AnyContext, types.Digest("11111111111111111111111111111111")).Return(image1, nil)
	mgc.On("GetImage", testutils.AnyContext, types.Digest("22222222222222222222222222222222")).Return(image2, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			GCSClient: mgc,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/diffs/11111111111111111111111111111111-22222222222222222222222222222222.png", nil)
	wh.ImageHandler(w, r)
	// The images are different in 1 channel per pixel. The first 4 pixels (lines) are a light
	// orange color, the last one is a light blue color (because it differs only in alpha).
	assertDiffImageWas(t, w, `! SKTEXTSIMPLE
1 5
0xfdd0a2ff
0xfdd0a2ff
0xfdd0a2ff
0xfdd0a2ff
0xc6dbefff`)
}

func TestImageHandler_OneUnknownImage_404Returned(t *testing.T) {
	unittest.SmallTest(t)

	image1 := loadAsPNGBytes(t, one_by_five.ImageOne)
	mgc := &mocks.GCSClient{}
	// These digests are arbitrary - they do not match the provided images.
	mgc.On("GetImage", testutils.AnyContext, types.Digest("11111111111111111111111111111111")).Return(image1, nil)
	mgc.On("GetImage", testutils.AnyContext, types.Digest("22222222222222222222222222222222")).Return(nil, errors.New("unknown"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			GCSClient: mgc,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/diffs/11111111111111111111111111111111-22222222222222222222222222222222.png", nil)
	wh.ImageHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestImageHandler_TwoUnknownImages_404Returned(t *testing.T) {
	unittest.SmallTest(t)

	mgc := &mocks.GCSClient{}
	mgc.On("GetImage", testutils.AnyContext, types.Digest("11111111111111111111111111111111")).Return(nil, errors.New("unknown"))
	mgc.On("GetImage", testutils.AnyContext, types.Digest("22222222222222222222222222222222")).Return(nil, errors.New("unknown"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			GCSClient: mgc,
		},
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/diffs/11111111111111111111111111111111-22222222222222222222222222222222.png", nil)
	wh.ImageHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestImageHandler_InvalidRequest_404Returned(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/diffs/not_valid.png", nil)
	wh.ImageHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func TestImageHandler_InvalidImageFormat_404Returned(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/img/images/0123456789abcdef0123456789abcdef.gif", nil)
	wh.ImageHandler(w, r)
	assert.Equal(t, http.StatusNotFound, w.Result().StatusCode)
}

func loadAsPNGBytes(t *testing.T, textImage string) []byte {
	img := text.MustToNRGBA(textImage)
	var buf bytes.Buffer
	require.NoError(t, encodeImg(&buf, img))
	return buf.Bytes()
}

func TestChangelistSummaryHandler_ValidInput_CorrectJSONReturned(t *testing.T) {
	unittest.SmallTest(t)

	ms := &mock_search2.API{}
	ms.On("NewAndUntriagedSummaryForCL", testutils.AnyContext, "my-system_my_cl").Return(search2.NewAndUntriagedSummary{
		ChangelistID: "my_cl",
		PatchsetSummaries: []search2.PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   2,
			TotalUntriagedImages: 3,
			PatchsetID:           "patchset1",
			PatchsetOrder:        1,
		}, {
			NewImages:            5,
			NewUntriagedImages:   6,
			TotalUntriagedImages: 7,
			PatchsetID:           "patchset8",
			PatchsetOrder:        8,
		}},
		LastUpdated: time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC),
	}, nil)
	ms.On("ChangelistLastUpdated", testutils.AnyContext, "my-system_my_cl").Return(time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC), nil)

	wh := initCaches(Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: ms,
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"id":     "my_cl",
		"system": "my-system",
	})
	wh.ChangelistSummaryHandler(w, r)
	// Note this JSON had the patchsets sorted so the latest one is first.
	const expectedJSON = `{"changelist_id":"my_cl","patchsets":[{"new_images":5,"new_untriaged_images":6,"total_untriaged_images":7,"patchset_id":"patchset8","patchset_order":8},{"new_images":1,"new_untriaged_images":2,"total_untriaged_images":3,"patchset_id":"patchset1","patchset_order":1}],"outdated":false}`
	assertJSONResponseWas(t, http.StatusOK, expectedJSON, w)
}

func TestChangelistSummaryHandler_CachedValueStaleButUpdatesQuickly_ReturnsFreshResult(t *testing.T) {
	unittest.SmallTest(t)

	ms := &mock_search2.API{}
	// First call should have just one PS.
	ms.On("NewAndUntriagedSummaryForCL", testutils.AnyContext, "my-system_my_cl").Return(search2.NewAndUntriagedSummary{
		ChangelistID: "my_cl",
		PatchsetSummaries: []search2.PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   2,
			TotalUntriagedImages: 3,
			PatchsetID:           "patchset1",
			PatchsetOrder:        1,
		}},
		LastUpdated: time.Date(2021, time.March, 1, 1, 1, 1, 0, time.UTC),
	}, nil).Once()
	// Second call should have two PS and the latest timestamp.
	ms.On("NewAndUntriagedSummaryForCL", testutils.AnyContext, "my-system_my_cl").Return(search2.NewAndUntriagedSummary{
		ChangelistID: "my_cl",
		PatchsetSummaries: []search2.PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   2,
			TotalUntriagedImages: 3,
			PatchsetID:           "patchset1",
			PatchsetOrder:        1,
		}, {
			NewImages:            5,
			NewUntriagedImages:   6,
			TotalUntriagedImages: 7,
			PatchsetID:           "patchset8",
			PatchsetOrder:        8,
		}},
		LastUpdated: time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC),
	}, nil).Once()
	ms.On("ChangelistLastUpdated", testutils.AnyContext, "my-system_my_cl").Return(time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC), nil)

	wh := initCaches(Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: ms,
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	})

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, requestURL, nil)
		r = mux.SetURLVars(r, map[string]string{
			"id":     "my_cl",
			"system": "my-system",
		})
		wh.ChangelistSummaryHandler(w, r)
		if i == 0 {
			continue
		}
		// Note this JSON had the patchsets sorted so the latest one is first.
		const expectedJSON = `{"changelist_id":"my_cl","patchsets":[{"new_images":5,"new_untriaged_images":6,"total_untriaged_images":7,"patchset_id":"patchset8","patchset_order":8},{"new_images":1,"new_untriaged_images":2,"total_untriaged_images":3,"patchset_id":"patchset1","patchset_order":1}],"outdated":false}`
		assertJSONResponseWas(t, http.StatusOK, expectedJSON, w)
	}
	ms.AssertExpectations(t)
}

func TestChangelistSummaryHandler_CachedValueStaleUpdatesSlowly_ReturnsStaleResult(t *testing.T) {
	unittest.SmallTest(t)

	ms := &mock_search2.API{}
	// First call should have just one PS.
	ms.On("NewAndUntriagedSummaryForCL", testutils.AnyContext, "my-system_my_cl").Return(search2.NewAndUntriagedSummary{
		ChangelistID: "my_cl",
		PatchsetSummaries: []search2.PatchsetNewAndUntriagedSummary{{
			NewImages:            1,
			NewUntriagedImages:   2,
			TotalUntriagedImages: 3,
			PatchsetID:           "patchset1",
			PatchsetOrder:        1,
		}},
		LastUpdated: time.Date(2021, time.March, 1, 1, 1, 1, 0, time.UTC),
	}, nil).Once()
	// Second call should have two PS and the latest timestamp.
	ms.On("NewAndUntriagedSummaryForCL", testutils.AnyContext, "my-system_my_cl").Return(func(context.Context, string) search2.NewAndUntriagedSummary {
		// This is longer than the time we wait before giving up and returning stale results.
		time.Sleep(2 * time.Second)
		return search2.NewAndUntriagedSummary{
			ChangelistID: "my_cl",
			PatchsetSummaries: []search2.PatchsetNewAndUntriagedSummary{{
				NewImages:            1,
				NewUntriagedImages:   2,
				TotalUntriagedImages: 3,
				PatchsetID:           "patchset1",
				PatchsetOrder:        1,
			}, {
				NewImages:            5,
				NewUntriagedImages:   6,
				TotalUntriagedImages: 7,
				PatchsetID:           "patchset8",
				PatchsetOrder:        8,
			}},
			LastUpdated: time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC),
		}
	}, nil).Once()
	ms.On("ChangelistLastUpdated", testutils.AnyContext, "my-system_my_cl").Return(time.Date(2021, time.April, 1, 1, 1, 1, 0, time.UTC), nil)

	wh := initCaches(Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: ms,
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	})

	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, requestURL, nil)
		r = mux.SetURLVars(r, map[string]string{
			"id":     "my_cl",
			"system": "my-system",
		})
		wh.ChangelistSummaryHandler(w, r)
		if i == 0 {
			continue
		}
		// Note this JSON is the first result marked as stale.
		const expectedJSON = `{"changelist_id":"my_cl","patchsets":[{"new_images":1,"new_untriaged_images":2,"total_untriaged_images":3,"patchset_id":"patchset1","patchset_order":1}],"outdated":true}`
		assertJSONResponseWas(t, http.StatusOK, expectedJSON, w)
	}
	ms.AssertExpectations(t)
}

func TestChangelistSummaryHandler_MissingCL_BadRequest(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"system": "my-system",
	})
	wh.ChangelistSummaryHandler(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestChangelistSummaryHandler_MissingSystem_BadRequest(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"id": "my_cl",
	})
	wh.ChangelistSummaryHandler(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestChangelistSummaryHandler_IncorrectSystem_BadRequest(t *testing.T) {
	unittest.SmallTest(t)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"id":     "my_cl",
		"system": "bad-system",
	})
	wh.ChangelistSummaryHandler(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
}

func TestChangelistSummaryHandler_SearchReturnsError_InternalServerError(t *testing.T) {
	unittest.SmallTest(t)

	ms := &mock_search2.API{}
	ms.On("ChangelistLastUpdated", testutils.AnyContext, "my-system_my_cl").Return(time.Time{}, errors.New("boom"))

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: ms,
			ReviewSystems: []clstore.ReviewSystem{{
				ID: "my-system",
			}},
		},
		anonymousGerritQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, requestURL, nil)
	r = mux.SetURLVars(r, map[string]string{
		"id":     "my_cl",
		"system": "my-system",
	})
	wh.ChangelistSummaryHandler(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Result().StatusCode)
}

func TestStartCacheWarming_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, datakitchensink.Build()))

	wh := initCaches(Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: search2.New(db, 10),
			DB:         db,
		},
	})

	// Set the time to be a few days after both CLs in the sample data land.
	ctx = context.WithValue(ctx, now.ContextKey, time.Date(2020, time.December, 14, 0, 0, 0, 0, time.UTC))
	wh.StartCacheWarming(ctx)
	require.Eventually(t, func() bool {
		return wh.clSummaryCache.Len() == 2
	}, 5*time.Second, 100*time.Millisecond)
	assert.True(t, wh.clSummaryCache.Contains("gerrit_CL_fix_ios"))
	assert.True(t, wh.clSummaryCache.Contains("gerrit-internal_CL_new_tests"))
}

func TestGetBlamesForUntriagedDigests_ValidInput_CorrectJSONReturned(t *testing.T) {
	unittest.SmallTest(t)

	ms := &mock_search2.API{}

	ms.On("GetBlamesForUntriagedDigests", testutils.AnyContext, "the_corpus").Return(search2.BlameSummaryV1{
		Ranges: []search2.BlameEntry{{
			CommitRange:           "commit04:commit05",
			TotalUntriagedDigests: 2,
			AffectedGroupings: []*search2.AffectedGrouping{{
				Grouping: paramtools.Params{
					types.CorpusField:     "the_corpus",
					types.PrimaryKeyField: "alpha",
				},
				UntriagedDigests: 1,
				SampleDigest:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			}, {
				Grouping: paramtools.Params{
					types.CorpusField:     "the_corpus",
					types.PrimaryKeyField: "beta",
				},
				UntriagedDigests: 1,
				SampleDigest:     "dddddddddddddddddddddddddddddddd",
			}},
			Commits: []frontend.Commit{{
				CommitTime: 12345678000,
				Hash:       "1234567890abcdef1234567890abcdef12345678",
				Author:     "user1@example.com",
				Subject:    "Probably broke something",
			}, {
				CommitTime: 12345678900,
				Hash:       "4567890abcdef1234567890abcdef1234567890a",
				Author:     "user2@example.com",
				Subject:    "Might not have broke anything",
			}},
		}}}, nil)

	wh := Handlers{
		HandlersConfig: HandlersConfig{
			Search2API: ms,
		},
		anonymousExpensiveQuota: rate.NewLimiter(rate.Inf, 1),
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/json/v2/byblame?query=source_type%3Dthe_corpus", nil)
	wh.ByBlameHandler2(w, r)
	const expectedJSON = `{"data":[{"groupID":"commit04:commit05","nDigests":2,"nTests":2,"affectedTests":[{"test":"alpha","num":1,"sample_digest":"bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},{"test":"beta","num":1,"sample_digest":"dddddddddddddddddddddddddddddddd"}],"commits":[{"commit_time":12345678000,"hash":"1234567890abcdef1234567890abcdef12345678","author":"user1@example.com","message":"Probably broke something","cl_url":""},{"commit_time":12345678900,"hash":"4567890abcdef1234567890abcdef1234567890a","author":"user2@example.com","message":"Might not have broke anything","cl_url":""}]}]}`
	assertJSONResponseWas(t, http.StatusOK, expectedJSON, w)
}

// Because we are calling our handlers directly, the target URL doesn't matter. The target URL
// would only matter if we were calling into the router, so it knew which handler to call.
const requestURL = "/does/not/matter"

// assertJSONResponseAndReturnBody asserts that the given ResponseRecorder was given the
// appropriate JSON and the expected status code, and returns the response body.
func assertJSONResponseAndReturnBody(t *testing.T, expectedStatusCode int, w *httptest.ResponseRecorder) []byte {
	resp := w.Result()
	assert.Equal(t, expectedStatusCode, resp.StatusCode)
	assert.Equal(t, jsonContentType, resp.Header.Get(contentTypeHeader))
	assert.Equal(t, allowAllOrigins, resp.Header.Get(accessControlHeader))
	assert.Equal(t, noSniffContent, resp.Header.Get(contentTypeOptionsHeader))
	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	return respBody
}

// assertJSONResponseWas asserts that the given ResponseRecorder was given the appropriate JSON
// headers and the expected status code and response body.
func assertJSONResponseWas(t *testing.T, expectedStatusCode int, expectedBody string, w *httptest.ResponseRecorder) {
	actualBody := assertJSONResponseAndReturnBody(t, expectedStatusCode, w)
	// The JSON encoder includes a newline "\n" at the end of the body, which is awkward to include
	// in the literals passed in above, so we add that here
	assert.Equal(t, expectedBody+"\n", string(actualBody))
}

func assertImageResponseWas(t *testing.T, expected []byte, w *httptest.ResponseRecorder) {
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	respBody, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, expected, respBody)
}

func assertDiffImageWas(t *testing.T, w *httptest.ResponseRecorder, expectedTextImage string) {
	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	respImg, err := decodeImg(resp.Body)
	require.NoError(t, err)
	var buf bytes.Buffer
	require.NoError(t, text.Encode(&buf, respImg))
	assert.Equal(t, expectedTextImage, buf.String())
}

// setID applies the ID mux.Var to a copy of the given request. In a normal server setting, mux will
// parse the given url with a string that indicates how to extract variables (e.g.
// '/json/ignores/save/{id}' and store those to the request's context. However, since we just call
// the handler directly, we need to set those variables ourselves.
func setID(r *http.Request, id string) *http.Request {
	return mux.SetURLVars(r, map[string]string{"id": id})
}

// waitForSystemTime waits for a time greater than the duration mentioned in "AS OF SYSTEM TIME"
// clauses in queries. This way, the queries will be accurate.
func waitForSystemTime() {
	time.Sleep(150 * time.Millisecond)
}

func initCaches(handlers Handlers) Handlers {
	clcache, err := lru.New(changelistSummaryCacheSize)
	if err != nil {
		panic(err)
	}
	handlers.clSummaryCache = clcache
	return handlers
}

// overwriteNow adds the provided time to the request's context (which is returned as a shallow
// copy of the original request).
func overwriteNow(r *http.Request, fakeNow time.Time) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), now.ContextKey, fakeNow))
}
