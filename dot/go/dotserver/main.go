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

const domain = "dot.skia.org"

const reportErrorDotTemplate = `
graph {
	%q [shape=box,fontcolor=red]
  }
`

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

func (srv *server) reportError(w http.ResponseWriter, err error, message string, statusCode int) {
	sklog.Error(message, err)
	svg, err := srv.tx(context.Background(), "dot", fmt.Sprintf(reportErrorDotTemplate, message))
	if err != nil {
		sklog.Error(err)
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.WriteHeader(statusCode)
	w.Write([]byte(svg))
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
	// Strip off leading slash in path.
	format := r.URL.Path[1:]

	if !util.In(format, validFormats) {
		srv.reportError(w, fmt.Errorf("Unknown format: %q", format), "Unknown format.", http.StatusNotFound)
		return
	}

	sourceURL := r.Header.Get("Referer")
	if sourceURL == "" {
		srv.reportError(w, fmt.Errorf("Missing Referer header."), "Missing Referer header.", http.StatusNotFound)
		return
	}

	// Load the HTML document
	resp, err := srv.client.Get(sourceURL)
	if err != nil {
		srv.reportError(w, fmt.Errorf("Failed to fetch referring page: %s", err), "Failed to fetch referring page.", http.StatusNotFound)
		return
	}
	defer util.Close(resp.Body)
	if resp.StatusCode != 200 {
		srv.reportError(w, fmt.Errorf("Failed to get 200 fetching referring page: %d", resp.StatusCode), "Failed to get 200 fetching referring page.", http.StatusNotFound)
		return
	}

	// Calc md5 hash of document.
	var buf bytes.Buffer
	// If there is an image in the cache for md5+url then return that else:
	// TODO(jcgregorio) Implement caching.
	_, err = util.MD5FromReader(resp.Body, &buf)
	if err != nil {
		srv.reportError(w, fmt.Errorf("Failed to calculate md5: %s", err), "Failed to calculate md5 of source page.", http.StatusNotFound)
		return
	}

	// Reconstruct the URL that initiated this request.
	// Sometimes URL.Host and Scheme are empty, if that's the case then
	// fill them in with correct values so we can reconstruct the request URL.
	if r.URL.Host == "" {
		r.URL.Host = domain
	}
	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}
	requestedURL := r.URL.String()

	// Parse the HTML.
	doc, err := goquery.NewDocumentFromReader(&buf)
	if err != nil {
		srv.reportError(w, fmt.Errorf("Failed to parse HTML document: %s", err), "Failed to parse source page.", http.StatusNotFound)
		return
	}
	found := false
	// We look for Graphviz data formatted in a specific way:
	//
	//  <details>
	//      <summary>
	//          <object type="image/svg+xml" data="https://dot.skia.org/dot"></object>
	//      </summary>
	//      <pre>
	//      graph {
	//          Hello -- World
	//      }
	//      </pre>
	//  </details>
	//
	// The details/summary allows for showing the summary, the generated SVG,
	// while hiding the dot code in a way that makes it easy to inspect it.
	//
	// We use an 'object' tag instead of an 'img' tag because that allows any
	// links in the SVG to be functional.
	//
	// The 'pre' tag makes it easy to grab the dot code and also formats the dot
	// code nicely.
	doc.Find("object").Each(func(i int, s *goquery.Selection) {
		if imgSrc, ok := s.Attr("data"); !ok || imgSrc != requestedURL {
			return
		}
		found = true
		dotCode := s.Parent().Parent().Find("pre").Text()
		svg, err := srv.tx(context.Background(), format, dotCode)
		if err != nil {
			srv.reportError(w, fmt.Errorf("Failed to transform: %s", err), "Failed to transform.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Write([]byte(svg))
		return
	})
	if !found {
		srv.reportError(w, fmt.Errorf("Couldn't find requested URL %q in source document %q", requestedURL, sourceURL), "Failed to find requester URL in source document.", http.StatusNotFound)
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
	baseapp.Serve(newServer, []string{domain})
}
