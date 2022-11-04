package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sser"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// flags
var (
	local         = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
	port          = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	sserPort      = flag.Int("sser_port", 7000, "Server-Sent Events peer connection port.")
	promPort      = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	namespace     = flag.String("namespace", "default", "Namespace this application is running in.")
	labelSelector = flag.String("label_selector", "app=sserexample", "Label selector for peers of this application.")
)

const page = `<!DOCTYPE html>
<html>
  <head>
    <title></title>
    <meta charset="utf-8" />
  </head>
  <body>
	<pre></pre>
  	<script type="text/javascript" charset="utf-8">
      const pre = document.querySelector('pre');
      const evtSource = new EventSource('/_/sse?stream=counter');
      const messages = [];
      evtSource.onmessage = (event) => {
        messages.push(event.data);
        while (messages.length > 10) {
          messages.shift();
        }
        pre.textContent = messages.join('\n');
      };
    </script>
  </body>
</html>
`

func index(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Context-Type", "text/html")
	_, err := w.Write([]byte(page))
	if err != nil {
		sklog.Errorf("write index page: %s", err)
	}
}

func main() {
	ctx := context.Background()
	common.InitWithMust(
		"sserexample",
		common.PrometheusOpt(promPort),
		common.MetricsLoggingOpt(),
	)

	var peerFinder sser.PeerFinder
	if !*local {
		config, err := rest.InClusterConfig()
		if err != nil {
			sklog.Fatal(err)
		}
		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			sklog.Fatal(err)
		}

		peerFinder, err = sser.NewPeerFinder(clientset, *namespace, *labelSelector)
		if err != nil {
			sklog.Fatal(err)
		}
	} else {
		peerFinder = sser.NewPeerFinderLocalhost()
	}

	sserServer, err := sser.New(*sserPort, peerFinder)
	if err != nil {
		sklog.Fatal(err)
	}
	err = sserServer.Start(ctx)
	if err != nil {
		sklog.Fatal(err)
	}

	r := mux.NewRouter()
	r.HandleFunc("/", index)
	r.HandleFunc("/_/sse", sserServer.ClientConnectionHandler(ctx))

	var h http.Handler = r
	if !*local {
		h = httputils.HealthzAndHTTPS(h)
	}

	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}

	count := 0
	go func() {
		for range time.Tick(time.Second) {
			count++
			sserServer.Send(ctx, "counter", fmt.Sprintf("%s - %d", hostname, count))
		}
	}()

	http.Handle("/", h)
	sklog.Info("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
