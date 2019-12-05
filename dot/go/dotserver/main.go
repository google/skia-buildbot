package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"regexp"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// flags
var (
	domain  = flag.String("domain", "dot.skia.org", "The domain this app is running on.")
	allowed = flag.String("allowed", `^https://(github.com/google/skia|skia.googlesource.com|(\w+.)?skia.org)`, "Regular expression that matches the URLs that are allowed to use this service.")
)

// The Graphviz formats we allow.
var validFormats = []string{"dot", "neato", "twopi", "circo", "fdp", "sfdp"}

// transformer is a func that transforms dot code into svg.
type transformer func(ctx context.Context, format string, dotCode string) (string, error)

// server implements base.App.
type server struct {
	client  *http.Client
	tx      transformer
	allowed *regexp.Regexp
}

func newServer() (baseapp.App, error) {
	return &server{
		client:  httputils.NewTimeoutClient(),
		tx:      transformToSVG,
		allowed: regexp.MustCompile(*allowed),
	}, nil
}

func (srv *server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	// TODO(jcgregorio) Fill in link to docs.
	_, err := w.Write([]byte(`<!DOCTYPE html>
<head>
  <style>
	p, h1 {
		font-family: sans-serif;
	}

	body {
		padding: 0 1rem 1rem 1rem;
	}

	pre {
		background: lightgray;
		color: darkgreen;
		padding: 1rem;
	}
  </style>
  <title>Dot</title>
</head>
<body>
<h1>Dot</h1>
<p>A service for transforming Graphviz data into SVG.</p>
<p>The Graphviz data must be formatted in a specific way:</p>
<pre>&lt;details>
  &lt;summary>
    &lt;object type="image/svg+xml" data="https://dot.skia.org/dot">&lt;/object>
  &lt;/summary>
  &lt;pre>
    graph {
	  Hello -- World
    }
    &lt;/pre>
&lt;/details></pre>

<p>
  Why this particular format? The details/summary allows for showing the summary,
  the generated SVG, while by default hiding the dot code, but in a way that makes
  it easy to view. We use an 'object' tag instead of an 'img' tag because that allows
  links in the SVG to be functional. The 'pre' tag makes it easy to grab the dot code
  and also formats the dot code nicely.
</p>

<p>
 Because &lt;object> tags are treated like iframes, all links in Graphviz should specify
 a target, for example:
</p>

<pre>
digraph {
	Jim [URL="https://www.google.com/" fillcolor="green4" style="filled" target="_blank"];
	Jim -> John;
	Jim -> Mary;
}
</pre>

<p>
  If you have more that one diagram on a singe page then make the 'data' URLs
  unique by adding to the query parameters. For example:
</p>

<pre>
  &lt;object type="image/svg+xml" data="https://dot.skia.org/dot?first-diagram">&lt;/object>

  ...

  &lt;object type="image/svg+xml" data="https://dot.skia.org/dot?second-diagram">&lt;/object>
</pre>

<p>
	The service understands the following formats:
</p>
<ul>
  <li>dot</li>
  <li>neato</li>
  <li>twopi</li>
  <li>circo</li>
  <li>fdp</li>
  <li>sfdp</li>
</ul>

<p>The format is specified via the path in the URL, for example to use neato:</p>

<pre>&lt;details>
&lt;summary>
  &lt;object type="image/svg+xml" data="https://dot.skia.org/neato">&lt;/object>
&lt;/summary>
&lt;pre>
  graph {
	  Hello -- World
  }
&lt;/pre>
&lt;/details></pre>

</body>
`))
	if err != nil {
		sklog.Errorf("Failed to render index page: %s", err)
	}
}

func transformToSVG(ctx context.Context, format, dotCode string) (string, error) {
	cmd := exec.CommandContext(ctx, format, "-Tsvg")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("Failed to create stdin pipe to dot: %s", err)
	}

	go func() {
		if _, err := io.WriteString(stdin, dotCode); err != nil {
			_ = stdin.Close()
			sklog.Errorf("Failed to write to dot stdin: %s", err)
			return
		}
		if err := stdin.Close(); err != nil {
			sklog.Errorf("Failed to close dot stdin: %s", err)
		}
	}()

	out, err := cmd.CombinedOutput()
	return string(out), err
}

func (srv *server) transformHandler(w http.ResponseWriter, r *http.Request) {
	// Strip off leading slash from path.
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

	if !srv.allowed.MatchString(sourceURL) {
		httputils.ReportError(w, fmt.Errorf("Not an allowed domain: %q", sourceURL), "Not an allowed domain.", http.StatusNotFound)
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

	// NOTE: If the service is too slow them implement caching by using md5
	// along with an lru in-memory cache. Also should use an HTTP caching client
	// to fetch pages.

	// Sometimes Host and Scheme are empty, fill them in so we can reconstuct
	// the requesting URL.
	if r.URL.Host == "" {
		r.URL.Host = *domain
	}
	if r.URL.Scheme == "" {
		r.URL.Scheme = "https"
	}
	requestedURL := r.URL.String()

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
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		httputils.ReportError(w, fmt.Errorf("Failed to parse HTML document: %s", err), "Failed to parse source page.", http.StatusNotFound)
		return
	}
	found := false // Only process the first matching response.
	doc.Find("object").Each(func(i int, s *goquery.Selection) {
		if found {
			return
		}
		if imgSrc, ok := s.Attr("data"); !ok || imgSrc != requestedURL {
			return
		}
		found = true
		dotCode := s.Parent().Parent().Find("pre").Text()
		svg, err := srv.tx(r.Context(), format, dotCode)
		if err != nil {
			httputils.ReportError(w, fmt.Errorf("Failed to transform: %s", err), "Failed to transform.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/svg+xml")
		// Make sure browsers don't cache the wrong value.
		w.Header().Set("Vary", "Referer")
		_, err = w.Write([]byte(svg))
		if err != nil {
			sklog.Errorf("Failed to write SVG: %s", err)
		}
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
	baseapp.Serve(newServer, []string{*domain})
}
