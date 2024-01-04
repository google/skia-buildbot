package backends

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"go.skia.org/infra/go/mockhttpclient"

	. "github.com/smartystreets/goconvey/convey"
	. "go.chromium.org/luci/common/testing/assertions"
)

func TestGetDeps(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	repo := "https://chromium.googlesource.com/chromium/src"
	hash := "12345"

	Convey(`E2E`, t, func() {
		Convey(`OK`, func() {
			urlMock := mockhttpclient.NewURLMock()
			c := NewSkiaBridgeClient(urlMock.Client())

			url := fmt.Sprintf(DepsURL, SkiaBridgeURL, repo, hash)

			r := map[string]string{
				"https://chromium.googlesource.com/v8/v8": "c092edb",
				"https://webrtc.googlesource.com/src":     "deadbeef",
			}
			js, _ := json.Marshal(r)
			urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(js)))

			resp, err := c.GetDeps(ctx, repo, hash)
			So(err, ShouldBeNil)
			So(resp, ShouldEqual, r)
		})

		Convey(`Change URL`, func() {
			urlMock := mockhttpclient.NewURLMock()
			newURL := "https://random.service"
			c := NewSkiaBridgeClient(urlMock.Client()).WithURL(newURL)

			url := fmt.Sprintf(DepsURL, newURL, repo, hash)

			r := map[string]string{
				"https://chromium.googlesource.com/v8/v8": "c092edb",
				"https://webrtc.googlesource.com/src":     "deadbeef",
			}
			js, _ := json.Marshal(r)
			urlMock.MockOnce(url, mockhttpclient.MockGetDialogue([]byte(js)))

			resp, err := c.GetDeps(ctx, repo, hash)
			So(err, ShouldBeNil)
			So(resp, ShouldEqual, r)
		})
	})

	Convey(`Error`, t, func() {
		Convey(`Non 200 From Dependency`, func() {
			urlMock := mockhttpclient.NewURLMock()
			c := NewSkiaBridgeClient(urlMock.Client())

			url := fmt.Sprintf(DepsURL, SkiaBridgeURL, repo, hash)

			urlMock.MockOnce(url, mockhttpclient.MockGetError("Not Found", 404))

			resp, err := c.GetDeps(ctx, repo, hash)
			So(err, ShouldErrLike, "Request returned status \"Not Found\"")
			So(resp, ShouldBeNil)
		})

		Convey(`Non JSON Parseable`, func() {
			urlMock := mockhttpclient.NewURLMock()
			c := NewSkiaBridgeClient(urlMock.Client())

			url := fmt.Sprintf(DepsURL, SkiaBridgeURL, repo, hash)

			urlMock.MockOnce(url, mockhttpclient.MockGetDialogue(nil))

			resp, err := c.GetDeps(ctx, repo, hash)
			So(err, ShouldErrLike, "unexpected end of JSON input")
			So(resp, ShouldBeNil)
		})
	})
}
