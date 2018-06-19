/*
expand-and-reload is a simple app that watches for a configmap file to change
and when it does it writes the file, after doing environment variable
expansion, to --dst.
*/
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/a8m/envsubst"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	fsnotify "gopkg.in/fsnotify.v1"
)

// flags
var (
	webhook  = flag.String("webhook-url", "", "The url to send a request to when the specified file has been updated.")
	src      = flag.String("src", "", "The name of the file to watch for changes.")
	dst      = flag.String("dst", "", "The src file with env variables expanded.")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':20000')")
)

func expand() error {
	buf, err := envsubst.ReadFile(*src)
	if err != nil {
		return fmt.Errorf("Failed to read or expand config-map: %s", err)
	}
	err = util.WithWriteFile(*dst, func(w io.Writer) error {
		_, err := w.Write(buf)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to write expanded config-map file: %s", err)
	}
	return nil
}

func main() {
	common.InitWithMust(
		filepath.Base(os.Args[0]),
		common.PrometheusOpt(promPort),
	)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		sklog.Fatal(err)
	}
	defer watcher.Close()

	err = watcher.Add(filepath.Dir(*src))
	if err != nil {
		sklog.Fatal(err)
	}

	client := httputils.NewTimeoutClient()

	if err := expand(); err != nil {
		sklog.Errorf("Failed to expand: %s", err)
	} else {
		sklog.Infof("Initial expansion done: %s", err)
	}
	for {
		select {
		case event := <-watcher.Events:
			sklog.Infof("Event: %v", event)
			if event.Op&fsnotify.Create == fsnotify.Create && filepath.Base(event.Name) == "..data" {
				sklog.Infof("config-map updated.")
				if err := expand(); err != nil {
					sklog.Errorf("Failed to expand: %s", err)
					continue
				}
				resp, err := client.Post(*webhook, "text/plain", nil)
				if err != nil {
					sklog.Errorf("Failed triggering webhook: %s", err)
					continue
				}
				util.Close(resp.Body)
				if resp.StatusCode != 200 {
					sklog.Errorf("Did not receive 200 status code: %d", resp.StatusCode)
					continue
				}
				sklog.Info("Successfully triggered reload.")
			}
		case err := <-watcher.Errors:
			sklog.Warningf("watcher error: %s", err)
		}
	}
}
