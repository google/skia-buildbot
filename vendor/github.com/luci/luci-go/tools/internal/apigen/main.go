// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package apigen

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"text/template"
	"time"

	"github.com/luci/luci-go/common/clock"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"github.com/luci/luci-go/common/sync/parallel"
	"golang.org/x/net/context"
)

const (
	defaultPackageBase = "github.com/luci/luci-go/common/api"

	// chromiumLicence is the standard Chromium license header.
	chromiumLicense = `` +
		"// Copyright {{.Year}} The LUCI Authors. All rights reserved.\n" +
		"// Use of this source code is governed under the Apache License, Version 2.0\n" +
		"// that can be found in the LICENSE file.\n" +
		"\n"
)

var (
	// chromiumLicenseTemplate is the compiled Chromium license template text.
	chromiumLicenseTemplate *template.Template
)

func init() {
	chromiumLicenseTemplate = template.Must(template.New("chromium license").Parse(chromiumLicense))
}

func compileChromiumLicense(c context.Context) (string, error) {
	buf := bytes.Buffer{}
	err := chromiumLicenseTemplate.Execute(&buf, map[string]interface{}{
		"Year": clock.Now(c).Year(),
	})
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Application is the main apigen application instance.
type Application struct {
	servicePath    string
	serviceAPIRoot string
	genPath        string
	apiPackage     string
	apiSubproject  string
	apiWhitelist   apiWhitelist
	baseURL        string

	license string
}

// AddToFlagSet adds application-level flags to the supplied FlagSet.
func (a *Application) AddToFlagSet(fs *flag.FlagSet) {
	flag.StringVar(&a.servicePath, "service", ".",
		"Path to the AppEngine service to generate from.")
	flag.StringVar(&a.serviceAPIRoot, "service-api-root", "/_ah/api/",
		"The service's API root path.")
	flag.StringVar(&a.genPath, "generator", "google-api-go-generator",
		"Path to the `google-api-go-generator` binary to use.")
	flag.StringVar(&a.apiPackage, "api-package", defaultPackageBase,
		"Name of the root API package on GOPATH.")
	flag.StringVar(&a.apiSubproject, "api-subproject", "",
		"If supplied, place APIs in an additional subdirectory under -api-package.")
	flag.Var(&a.apiWhitelist, "api",
		"If supplied, limit the emitted APIs to those named. Can be specified "+
			"multiple times.")
	flag.StringVar(&a.baseURL, "base-url", "http://localhost:8080",
		"Use this as the default base service client URL.")
}

func resolveExecutable(path *string) error {
	if path == nil || *path == "" {
		return errors.New("empty path")
	}
	lpath, err := exec.LookPath(*path)
	if err != nil {
		return fmt.Errorf("could not find [%s]: %s", *path, err)
	}

	st, err := os.Stat(lpath)
	if err != nil {
		return err
	}
	if st.Mode().Perm()&0111 == 0 {
		return errors.New("file is not executable")
	}
	*path = lpath
	return nil
}

// retryHTTP executes an HTTP call to the specified URL, retrying if it fails.
//
// It will return an error if no successful HTTP results were returned.
// Otherwise, it will return the body of the successful HTTP response.
func retryHTTP(c context.Context, u url.URL, method, body string) ([]byte, error) {
	client := http.Client{}

	gen := func() retry.Iterator {
		return &retry.Limited{
			Delay:   2 * time.Second,
			Retries: 20,
		}
	}

	output := []byte(nil)
	err := retry.Retry(c, gen, func() error {
		req := http.Request{
			Method: method,
			URL:    &u,
			Header: http.Header{},
		}
		if len(body) > 0 {
			req.Body = ioutil.NopCloser(bytes.NewBuffer([]byte(body)))
			req.ContentLength = int64(len(body))
			req.Header.Add("Content-Type", "application/json")
		}

		resp, err := client.Do(&req)
		if err != nil {
			return err
		}
		if resp.Body != nil {
			defer resp.Body.Close()
			output, err = ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
		}

		switch resp.StatusCode {
		case http.StatusOK, http.StatusNoContent:
			return nil

		default:
			return fmt.Errorf("unsuccessful status code (%d): %s", resp.StatusCode, resp.Status)
		}
	}, func(err error, d time.Duration) {
		log.Fields{
			log.ErrorKey: err,
			"url":        u.String(),
			"delay":      d,
		}.Infof(c, "Service is not up yet; retrying.")
	})
	if err != nil {
		return nil, err
	}

	log.Fields{
		"url": u.String(),
	}.Infof(c, "Service is alive!")
	return output, nil
}

// Run executes the application using the supplied context.
//
// Note that this intentionally consumes the Application by value, as we may
// modify its configuration as parameters become resolved.
func (a Application) Run(c context.Context) error {
	if err := resolveExecutable(&a.genPath); err != nil {
		return fmt.Errorf("invalid API generator path (-google-api-go-generator): %s", err)
	}

	apiDst, err := getPackagePath(a.apiPackage)
	if err != nil {
		return fmt.Errorf("failed to find package path for [%s]: %s", a.apiPackage, err)
	}
	if a.apiSubproject != "" {
		apiDst = augPath(apiDst, a.apiSubproject)
		a.apiPackage = strings.Join([]string{a.apiPackage, a.apiSubproject}, "/")
	}
	log.Fields{
		"package": a.apiPackage,
		"path":    apiDst,
	}.Debugf(c, "Identified API destination package path.")

	// Compile our Chromium license.
	a.license, err = compileChromiumLicense(c)
	if err != nil {
		return fmt.Errorf("failed to compile Chromium license: %s", err)
	}

	c, cancelFunc := context.WithCancel(c)
	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)
	go func() {
		for range sigC {
			cancelFunc()
		}
	}()
	defer signal.Stop(sigC)

	// (1) Execute our service. Capture its discovery API.
	svc, err := loadService(c, a.servicePath)
	if err != nil {
		return fmt.Errorf("failed to load service [%s]: %s", a.servicePath, err)
	}

	err = svc.run(c, func(c context.Context, discoveryURL url.URL) error {
		discoveryURL.Path = safeURLPathJoin(discoveryURL.Path, a.serviceAPIRoot, "discovery", "v1", "apis")

		data, err := retryHTTP(c, discoveryURL, "GET", "")
		if err != nil {
			return fmt.Errorf("discovery server did not come online: %s", err)
		}

		dir := directoryList{}
		if err := json.Unmarshal(data, &dir); err != nil {
			return fmt.Errorf("failed to load directory list: %s", err)
		}

		// Ensure that our target API base directory exists.
		if err := ensureDirectory(apiDst); err != nil {
			return fmt.Errorf("failed to create destination directory: %s", err)
		}

		// Run "google-api-go-generator" against the hosted service.
		err = parallel.FanOutIn(func(taskC chan<- func() error) {
			for i, item := range dir.Items {
				item := item
				c := log.SetFields(c, log.Fields{
					"index": i,
					"api":   item.ID,
				})

				if !a.isWhitelisted(item.ID) {
					log.Infof(c, "API is not whitelisted; skipping.")
					continue
				}

				taskC <- func() error {
					return a.generateAPI(c, item, &discoveryURL, apiDst)
				}
			}
		})
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		log.Fields{
			log.ErrorKey: err,
		}.Errorf(c, "Failed to extract APIs.")
	}

	return nil
}

// generateAPI generates and installs a single directory item's API.
func (a *Application) generateAPI(c context.Context, item *directoryItem, discoveryURL *url.URL, dst string) error {
	tmpdir, err := ioutil.TempDir(os.TempDir(), "apigen")
	if err != nil {
		return err
	}
	defer func() {
		os.RemoveAll(tmpdir)
	}()

	gendir := augPath(tmpdir, "gen")
	headerPath := augPath(tmpdir, "header.txt")
	if err := ioutil.WriteFile(headerPath, []byte(a.license), 0644); err != nil {
		return err
	}

	args := []string{
		"-cache=false", // Apparently the form {"-cache", "false"} is ignored.
		"-discoveryurl", discoveryURL.String(),
		"-api", item.ID,
		"-gendir", gendir,
		"-api_pkg_base", a.apiPackage,
		"-base_url", a.baseURL,
		"-header_path", headerPath,
	}
	log.Fields{
		"command": a.genPath,
		"args":    args,
	}.Debugf(c, "Executing google-api-go-generator.")
	out, err := exec.Command(a.genPath, args...).CombinedOutput()
	log.Infof(c, "Output:\n%s", out)
	if err != nil {
		return fmt.Errorf("error executing google-api-go-generator: %s", err)
	}

	err = installSource(gendir, dst, func(relpath string, data []byte) ([]byte, error) {
		// Skip the root "api-list.json" file. This is generated only for the subset
		// of APIs that this installation is handling, and is not representative of
		// the full discovery (much less installation) API set.
		if relpath == "api-list.json" {
			return nil, nil
		}

		if !strings.HasSuffix(relpath, "-gen.go") {
			return data, nil
		}

		log.Fields{
			"relpath": relpath,
		}.Infof(c, "Fixing up generated Go file.")
		return data, nil
	})
	if err != nil {
		return fmt.Errorf("failed to install [%s]: %s", item.ID, err)
	}
	return nil
}

func (a *Application) isWhitelisted(id string) bool {
	if len(a.apiWhitelist) == 0 {
		return true
	}
	for _, w := range a.apiWhitelist {
		if w == id {
			return true
		}
	}
	return false
}

func safeURLPathJoin(p ...string) string {
	for i, v := range p {
		p[i] = strings.Trim(v, "/")
	}
	return strings.Join(p, "/")
}
