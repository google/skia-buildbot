// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"flag"
	"time"

	"github.com/luci/luci-go/common/tsmon/target"
)

// GCECredentials is special value that can be passed instead of path to
// credentials file to indicate that app assertion credentials should be used
// instead of a real credentials file.
const (
	GCECredentials = ":gce"
)

// Flags defines command line flags related to tsmon.  Use NewFlags()
// to get a Flags struct with sensible default values.
type Flags struct {
	ConfigFile    string
	Endpoint      string
	Credentials   string
	Flush         FlushType
	FlushInterval time.Duration

	Target target.Flags
}

// NewFlags returns a Flags struct with sensible default values.
func NewFlags() Flags {
	return Flags{
		ConfigFile:    defaultConfigFilePath(),
		Endpoint:      "",
		Credentials:   "",
		Flush:         FlushAuto,
		FlushInterval: time.Minute,

		Target: target.NewFlags(),
	}
}

// Register adds tsmon related flags to a FlagSet.
func (fl *Flags) Register(f *flag.FlagSet) {
	f.StringVar(&fl.ConfigFile, "ts-mon-config-file", fl.ConfigFile,
		"path to a JSON config file that contains suitable values for "+
			"\"endpoint\" and \"credentials\" for this machine. This config file is "+
			"intended to be shared by all processes on the machine, as the values "+
			"depend on the machine's position in the network, IP whitelisting and "+
			"deployment of credentials.")
	f.StringVar(&fl.Endpoint, "ts-mon-endpoint", fl.Endpoint,
		"url (including file://, pubsub://project/topic) to post monitoring "+
			"metrics to. If set, overrides the value in --ts-mon-config-file")
	f.StringVar(&fl.Credentials, "ts-mon-credentials", fl.Credentials,
		"path to a pkcs8 json credential file. If set, overrides the value in "+
			"--ts-mon-config-file")
	f.Var(&fl.Flush, "ts-mon-flush",
		"metric push behavior: manual (only send when Flush() is called), or auto "+
			"(send automatically every --ts-mon-flush-interval)")
	f.DurationVar(&fl.FlushInterval, "ts-mon-flush-interval", fl.FlushInterval,
		"automatically push metrics on this interval if --ts-mon-flush=auto")

	fl.Target.Register(f)
}
