// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package isolatedclient

import (
	"errors"
	"flag"
	"net/http/httptest"
	"os"

	"github.com/luci/luci-go/common/isolatedclient/isolatedfake"
	"github.com/luci/luci-go/common/lhttp"
)

// Flags contains values parsed from command line arguments.
type Flags struct {
	ServerURL string
	Namespace string
}

// Init registers flags in a given flag set.
func (c *Flags) Init(f *flag.FlagSet) {
	i := os.Getenv("ISOLATE_SERVER")
	f.StringVar(&c.ServerURL, "isolate-server", i,
		"Isolate server to use; defaults to value of $ISOLATE_SERVER; use special value 'fake' to use a fake server")
	f.StringVar(&c.ServerURL, "I", i, "Alias for -isolate-server")
	f.StringVar(&c.Namespace, "namespace", DefaultNamespace, "")
}

// Parse applies changes specified by command line flags.
func (c *Flags) Parse() error {
	if c.ServerURL == "" {
		return errors.New("-isolate-server must be specified")
	}
	if c.ServerURL == "fake" {
		ts := httptest.NewServer(isolatedfake.New())
		c.ServerURL = ts.URL
	} else {
		s, err := lhttp.CheckURL(c.ServerURL)
		if err != nil {
			return err
		}
		c.ServerURL = s
	}
	if c.Namespace == "" {
		return errors.New("-namespace must be specified")
	}
	return nil
}
