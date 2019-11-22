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
	format := r.URL.Path
	if len(format) <= 1 {
		httputils.ReportError(w, fmt.Errorf("Invalid format: %q", format), "Invalid format.", http.StatusNotFound)
		return
	}

	// Strip off leading slash.
	format = format[1:]

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
	// md5 hash it
	resp, err := srv.client.Get(sourceURL)
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to fetch referring page: %s", err), "Failed to fetch referring page.", http.StatusNotFound)
		return
	}
	defer util.Close(resp.Body)
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
	requestedURL := r.URL.String()
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
		httputils.ReportError(w, fmt.Errorf("Couldn't find requested URL %q in source document %q", requestedURL, sourceURL), "Failed to calculate md5 of source page.", http.StatusNotFound)
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
