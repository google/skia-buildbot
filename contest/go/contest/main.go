package main

import (
	"flag"
	"html/template"
	"net/http"
	"strings"
	"sync"
	"time"

	"google.golang.org/api/sheets/v4"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
)

const (
	index = `<!DOCTYPE html>
<html>
<head>
    <title>Skia Drawing Contest</title>
    <meta charset="utf-8" />
    <meta http-equiv="X-UA-Compatible" content="IE=egde,chrome=1">
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
  <h2>Skia Coding Contest</h2>
  <p>
	  Create your entry at <a href="https://fiddle.skia.org">fiddle.skia.org</a> and submit contest entries <a href="http://goo.gl/forms/pRfo39hTND">here</a>.
	</p>
  {{range .}}
	  <div>
			<div class=entry>
				<a href='https://fiddle.skia.org/c/{{.Hash}}'>
					<video autoplay loop src='https://fiddle.skia.org/i/{{.Hash}}_cpu.webm' poster='https://fiddle.skia.org/i/{{.Hash}}_raster.png'></video>
				</a>
				<div style='padding-top: 1em;'><b>{{.Name}}</b></div>
			</div>
	  </div>
	{{end}}
	<script type="text/javascript" charset="utf-8">
		if (window.location.search.indexOf("refresh") != -1) {
			window.setTimeout(function() { window.location.reload(true); }, 60*1000);
		}
	</script>
</body>
</html>`
)

// flags
var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
)

type Values struct {
	Hash string
	Name string
}

var (
	indexTemplate = template.Must(template.New("index").Parse(index))
	ss            *sheets.Service
	values        []*Values
	mutex         sync.Mutex
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := indexTemplate.Execute(w, values); err != nil {
		sklog.Errorf("Failed to expand template: %s", err)
	}
}

func step() error {
	v, err := ss.Spreadsheets.Values.Get("1Jbv7pWwH8NwHtoEBez1Bkwmk2wjhMnCk9UKttYqPjNQ", "A1:C100").Do()
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
	sklog.Infof("%#v", values)
	return nil
}

func main() {
	defer common.LogPanic()
	common.InitWithMust(
		"contest",
		common.PrometheusOpt(promPort),
		common.CloudLoggingOpt(),
	)

	client, err := auth.NewDefaultJWTServiceAccountClient(sheets.SpreadsheetsReadonlyScope)
	if err != nil {
		sklog.Fatalf("Failed to auth: %s", err)
	}
	ss, err = sheets.New(client)
	if err != nil {
		sklog.Fatalf("Failed to create Sheets client: %s", err)
	}

	if err := step(); err != nil {
		sklog.Fatalf("Failed initial population of contest entries: %s", err)
	}
	go func() {
		for _ = range time.Tick(time.Minute) {
			if err := step(); err != nil {
				sklog.Fatalf("Failed to refresh from Google Sheets: %s", err)
			}
		}
	}()

	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
