package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/oauth2/google"
	sheets "google.golang.org/api/sheets/v4"
)

const (
	index = `<!DOCTYPE html>
<html>
<head>
    <title>Skia Drawing Gallery</title>
    <meta charset="utf-8" />
    <meta http-equiv="X-UA-Compatible" content="IE=edge">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
		<style>
		  body {
				color: #000;
				background: #FFF;
				font-family: Helvetica,Arial,'Bitstream Vera Sans',sans-serif;
				margin: 0;
				padding: 0;
			}

			.entry {
				text-align: center;
				border: solid lightgray 1px;
				padding: 1em;
				margin: 1em;
				float: left;'
			}

			a {
				color: #1f78b4;
			}

			h2 {
				background: #1f78b4;
				color: white;
				padding: 1em;
				margin: 0;
			}

			p {
				margin: 1em;
			}
		</style>
</head>
<body>
  <h2>Skia Coding Gallery</h2>
  <p>
	  Create your entry at <a target=_blank href="https://fiddle.skia.org">fiddle.skia.org</a> and submit the URL of your completed fiddle <a href="https://forms.gle/XSzwGQisp8UsXk5K8">here</a>.
	</p>
  {{range .Values}}
	  <div>
			<div class=entry>
				<a href='https://fiddle.skia.org/c/{{.Hash}}'>
					<video autoplay loop width=256 height=256 src='https://fiddle.skia.org/i/{{.Hash}}_cpu.webm' poster='https://fiddle.skia.org/i/{{.Hash}}_raster.png'></video>
				</a>
				<div style='padding-top: 1em;'><b>{{.Name}}</b></div>
			</div>
	  </div>
	{{end}}
	<script type="text/javascript" charset="utf-8">
	  // Hit /update and check for an updated hash value and if it has changed then refresh the page.
		function checkForUpdates() {
			var xhr = new XMLHttpRequest();
			xhr.open("GET", "/update", true);
			xhr.onreadystatechange = function(e) {
				if (e.currentTarget.readyState === XMLHttpRequest.DONE) {
					if (e.currentTarget.responseText === "{{.Hash}}") {
						window.setTimeout(checkForUpdates, 5000);
					} else {
						window.location.reload(true);
					}
				}
			};
			xhr.send();
		}

		checkForUpdates();
	</script>
</body>
</html>`
)

// flags
var (
	local    = flag.Bool("local", false, "Running locally, not in prod.")
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

// Values contains the fiddle hash and the user's name of a single contest entry.
type Values struct {
	Hash string
	Name string
}

// Context is used to expand the HTML template.
type Context struct {
	Values []*Values
	Hash   string
}

var (
	indexTemplate = template.Must(template.New("index").Parse(index))
	ss            *sheets.Service

	// mutex protects values and valueHash.
	mutex      sync.Mutex
	values     []*Values
	valuesHash string // valueHash is an md5 hash of values.
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	w.Header().Set("Content-Type", "text/html")
	if err := indexTemplate.Execute(w, Context{
		Values: values,
		Hash:   valuesHash,
	}); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

// updateHandler returns valuesHash.
func updateHandler(w http.ResponseWriter, r *http.Request) {
	mutex.Lock()
	defer mutex.Unlock()
	if _, err := w.Write([]byte(valuesHash)); err != nil {
		sklog.Errorf("Failed to write hash response: %s", err)
	}
}

// hash computes an md5 hash of date in 'values'.
func hash(values []*Values) string {
	h := md5.New()
	for _, v := range values {
		_, _ = h.Write([]byte(v.Hash))
		_, _ = h.Write([]byte(v.Name))
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

// step does a single update of 'values' and 'valueHash' from the Google spreadsheet.
func step() error {
	v, err := ss.Spreadsheets.Values.Get("1r0-HUMcZfeVS8SqaDDUZyrUJZy0AQ8lHhmWCI6gIlMw", "A1:C200").Do()
	if err != nil {
		return err
	}
	// Should parse out the hash from the URL, also drop the first row since that's the header.
	newValues := []*Values{}
	for _, slice := range v.Values {
		parts := strings.Split(slice[1].(string), "/")
		if len(parts) != 5 {
			continue
		}
		hash := parts[4]
		if hash == "" {
			continue
		}
		newValues = append(newValues, &Values{
			Hash: hash,
			Name: slice[2].(string),
		})
	}
	mutex.Lock()
	defer mutex.Unlock()
	values = newValues
	valuesHash = hash(values)
	return nil
}

func main() {
	common.InitWithMust(
		"contest",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	ts, err := google.DefaultTokenSource(context.Background(), sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	client := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	ss, err = sheets.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create Sheets client: %s", err)
	}

	if err := step(); err != nil {
		sklog.Fatalf("Failed initial population of contest entries: %s", err)
	}
	go func() {
		for range time.Tick(15 * time.Second) {
			if err := step(); err != nil {
				sklog.Fatalf("Failed to refresh from Google Sheets: %s", err)
			}
		}
	}()
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/update", updateHandler)
	h := httputils.LoggingGzipRequestResponse(r)
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}
	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
