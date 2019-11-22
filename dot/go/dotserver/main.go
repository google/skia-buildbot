package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

type transformer func(context.Context, string, string) (string, error)

// server implements base.App.
type server struct {
	client *http.Client
	tx     transformer
}

func newServer() (baseapp.App, error) {
	return &server{
		client: httputils.NewTimeoutClient(),
		tx:     transformToSVG,
	}, nil
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<body>
  <p>See https://... for how to use this service.</p>
</body>
`))
}

func transformToSVG(ctx context.Context, format, dotCode string) (string, error) {
	cmd := exec.CommandContext(ctx, format, "-Tsvg")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("Failed to create stdin pipe to dot: %s", err)
	}

	go func() {
		defer stdin.Close()
		_, err := io.WriteString(stdin, dotCode)
		if err != nil {
			sklog.Errorf("Failed to write to dot stdin: %s", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

var validFormats = []string{"dot", "neato", "twopi", "circo", "fdp", "sfdp"}

func (srv *server) transformHandler(w http.ResponseWriter, r *http.Request) {
	// Strip off leading slash.
	format := r.URL.Path[1:]

	if !util.In(format, validFormats) {
		httputils.ReportError(w, fmt.Errorf("Unknown format: %q", format), "Unknown format.", http.StatusNotFound)
		return
	}

	sourceURL := r.Header.Get("Referer")
	if sourceURL == "" {
		httputils.ReportError(w, fmt.Errorf("Missing Referer header."), "Missing Referer header.", http.StatusNotFound)
		return
	}
	// Load the HTML document
	resp, err := srv.client.Get(sourceURL)
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to fetch referring page: %s", err), "Failed to fetch referring page.", http.StatusNotFound)
		return
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		httputils.ReportError(w, fmt.Errorf("Failed to get 200 fetching referring page: %d", resp.StatusCode), "Failed to get 200 fetching referring page.", http.StatusNotFound)
		return
	}

	var buf bytes.Buffer
	// If there is an image in the cache for md5+url then return that else:
	// TODO(jcgregorio) use md5 along with lru cache.
	_, err = util.MD5FromReader(resp.Body, &buf)
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to calculate md5: %s", err), "Failed to calculate md5 of source page.", http.StatusNotFound)
		return
	}

	// Load the referring document, parse HTML, find the img tag with the URL we
	// are currently servicing, find the nearby dot code, turn it into SVG,
	// store in the cache, and return the SVG.
	requestedURL := r.RequestURI
	sklog.Infof("%#v %#v", *r, *(r.URL))

	/*
	 http.Request{Method:"GET", URL:(*url.URL)(0xc0001ba700), Proto:"HTTP/1.1", ProtoMajor:1, ProtoMinor:1, Header:http.Header{"Accept":[]string{"image/webp,image/apng,image/*,;q=0.8"}, "Accept-Encoding":[]string{"gzip, deflate, br"}, "Accept-Language":[]string{"en-US,en;q=0.9,la;q=0.8"}, "Cache-Control":[]string{"no-cache"}, "Content-Length":[]string{"0"}, "Cookie":[]string{"sksession=044A934F78AE87EE8EE1485F6C7C6B76; _ga=GA1.2.1172798559.1571341866; sktoken=MTU3NDA4ODM3OHxQXy1CQXdFQkIxTmxjM05wYjI0Ql80SUFBUVFCQlVWdFlXbHNBUXdBQVFKSlJBRU1BQUVKUVhWMGFGTmpiM0JsQVF3QUFRVlViMnRsYmdIX2hBQUFBRTdfZ3dNQkFRVlViMnRsYmdIX2hBQUJCQUVMUVdOalpYTnpWRzlyWlc0QkRBQUJDVlJ2YTJWdVZIbHdaUUVNQUFFTVVtVm1jbVZ6YUZSdmEyVnVBUXdBQVFaRmVIQnBjbmtCXzRZQUFBQVFfNFVGQVFFRVZHbHRaUUhfaGdBQUFQX2ZfNElCRldwalozSmxaMjl5YVc5QVoyOXZaMnhsTG1OdmJRRVZNVEV3TmpReU1qVTVPVGcwTlRrNU5qUTFPREV6QVFWbGJXRnBiQUVCXzRsNVlUSTVMa2x0UjNoQ04ydzFjbmRCUkhWZlpqYzVZamg0ZHpVeVFuQk1NR2RJUkdabFZFOHdVM010V1dGV2NGTlBNV05sWlV0QmQyOWZMVTFuU0MweE1FdEpUbUpPTmtaeU5XMHdaVnBOVUV0MFNGYzRTR0ZUVTBkMVRGTlRkbEJDVEZKRlZqWnlha3Q0YVRscU4xVkljMHBGUkhsbWF6STVZVzVmVGtabVprRXpVVXN5Y25keFlnRUdRbVZoY21WeUFnOEJBQUFBRHRWa3M4b3NieTRzQUFBQUFBPT18mbXsgA6lC2MJpookQERXFvJzUMKvSY9rr_PE5rcyT1Y=; _gid=GA1.2.1006267082.1574458001"}, "Pragma":[]string{"no-cache"}, "Referer":[]string{"https://skia.org/?cl=256098"}, "Sec-Fetch-Mode":[]string{"no-cors"}, "Sec-Fetch-Site":[]string{"same-site"}, "Strict-Transport-Security":[]string{"max-age=31536000; preload;"}, "User-Agent":[]string{"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/78.0.3904.108 Safari/537.36"}, "Via":[]string{"1.1 google"}, "X-Cloud-Trace-Context":[]string{"b2defa52f9e3d2e89286953337eb6cd4/14854289028407957877"}, "X-Content-Type-Options":[]string{"nosniff"}, "X-Envoy-Expected-Rq-Timeout-Ms":[]string{"600000"}, "X-Forwarded-For":[]string{"104.132.164.65, 35.201.76.220"}, "X-Forwarded-Proto":[]string{"https"}, "X-Request-Id":[]string{"9e6a25f5-2847-4810-804b-048a4dd7360b"}, "X-Xss-Protection":[]string{"1; mode=block"}}, Body:http.noBody{}, GetBody:(func() (io.ReadCloser, error))(nil), ContentLength:0, TransferEncoding:[]string(nil), Close:false, Host:"dot.skia.org", Form:url.Values(nil), PostForm:url.Values(nil), MultipartForm:(*multipart.Form)(nil), Trailer:http.Header(nil), RemoteAddr:"10.40.7.142:54564", RequestURI:"/dot?foo", TLS:(*tls.ConnectionState)(nil), Cancel:(<-chan struct {})(nil), Response:(*http.Response)(nil), ctx:(*context.valueCtx)(0xc000597170)} url.URL{Scheme:"", Opaque:"", User:(*url.Userinfo)(nil), Host:"", Path:"/dot", RawPath:"", ForceQuery:false, RawQuery:"foo", Fragment:""}

	*/

	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to parse HTML document: %s", err), "Failed to parse source page.", http.StatusNotFound)
		return
	}
	found := false
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		if imgSrc, ok := s.Attr("src"); !ok || imgSrc != requestedURL {
			return
		}
		found = true
		dotCode := s.Parent().Parent().Find("pre").Text()
		svg, err := srv.tx(context.Background(), format, dotCode)
		if err != nil {
			httputils.ReportError(w, fmt.Errorf("Failed to transform: %s", err), "Failed to transform.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/svg")
		w.Write([]byte(svg))
		return
	})
	if !found {
		httputils.ReportError(w, fmt.Errorf("Couldn't find requested URL %q in source document %q", requestedURL, sourceURL), "Failed to find requester URL in source document.", http.StatusNotFound)
	}
}

// See baseapp.App.
func (srv *server) AddHandlers(r *mux.Router) {
	r.HandleFunc("/", srv.indexHandler)
	r.HandleFunc("/{[a-z]+}", srv.transformHandler)
}

// See baseapp.App.
func (srv *server) AddMiddleware() []mux.MiddlewareFunc {
	return []mux.MiddlewareFunc{}
}

func main() {
	baseapp.Serve(newServer, []string{"dot.skia.org"})
}
