package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/docsyserver/go/codereview"
	"go.skia.org/infra/docsyserver/go/docset/mocks"
	"go.skia.org/infra/go/testutils"
)

func TestMainHandler_ValidCL(t *testing.T) {
	dir := t.TempDir()
	for _, cl := range []string{
		"",
		"main",
		"12345",
		"I8473b95934b5732ac55d26311a706c9c2bde9940",
		"myProject~12345",
		"chromium%2Fsrc~12345",
		"myProject~main~I8473b95934b5732ac55d26311a706c9c2bde9940",
		"chromium%2Fsrc~main~I8473b95934b5732ac55d26311a706c9c2bde9940",
	} {
		t.Run("cl_"+cl, func(t *testing.T) {
			expectedIssue := codereview.Issue(cl)
			if expectedIssue == "" {
				expectedIssue = codereview.MainIssue
			}
			ds := mocks.NewDocSet(t)
			ds.On("FileSystem", testutils.AnyContext, expectedIssue).Return(http.Dir(dir), nil).Once()
			s := &server{docset: ds}

			req := httptest.NewRequest(http.MethodGet, "/?cl="+url.QueryEscape(cl), nil)
			w := httptest.NewRecorder()
			s.mainHandler(w, req)

			require.Equal(t, http.StatusOK, w.Result().StatusCode)
		})
	}
}

func TestMainHandler_InvalidCL(t *testing.T) {
	for _, invalidCL := range []string{
		"../accounts/self/sshkeys?",
		"../accounts/self/sshkeys",
		"123/../../accounts/self",
		"%2e%2e/accounts/self",
		"project/branch~123",
	} {
		t.Run("query_"+invalidCL, func(t *testing.T) {
			ds := mocks.NewDocSet(t)
			s := &server{docset: ds}

			req := httptest.NewRequest(http.MethodGet, "/?cl="+invalidCL, nil)
			w := httptest.NewRecorder()
			s.mainHandler(w, req)

			require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
			ds.AssertNotCalled(t, "FileSystem", testutils.AnyContext, testutils.AnyContext)
		})

		t.Run("referer_"+invalidCL, func(t *testing.T) {
			ds := mocks.NewDocSet(t)
			s := &server{docset: ds}

			req := httptest.NewRequest(http.MethodGet, "/image.png", nil)
			req.Header.Set("Referer", "https://skia.org/?cl="+invalidCL)
			w := httptest.NewRecorder()
			s.mainHandler(w, req)

			require.Equal(t, http.StatusBadRequest, w.Result().StatusCode)
			ds.AssertNotCalled(t, "FileSystem", testutils.AnyContext, testutils.AnyContext)
		})
	}
}
