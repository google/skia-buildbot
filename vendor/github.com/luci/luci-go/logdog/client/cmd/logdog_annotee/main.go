// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/clock/clockflag"
	"github.com/luci/luci-go/common/config"
	log "github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/logging/gologger"
	"github.com/luci/luci-go/common/proto/milo"
	"github.com/luci/luci-go/logdog/client/annotee"
	"github.com/luci/luci-go/logdog/client/annotee/executor"
	"github.com/luci/luci-go/logdog/client/bootstrapResult"
	"github.com/luci/luci-go/logdog/client/butlerlib/bootstrap"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamclient"
	"github.com/luci/luci-go/logdog/client/butlerlib/streamproto"
	"github.com/luci/luci-go/logdog/common/types"
	"golang.org/x/net/context"
)

const (
	// configErrorReturnCode is returned when there is an error with Annotee's
	// command-line configuration.
	configErrorReturnCode = 2

	// runtimeErrorReturnCode is returned when the execution fails due to an
	// Annotee runtime error. This is intended to help differentiate Annotee
	// errors from passthrough bootstrapped subprocess errors.
	//
	// This will only be returned for runtime errors. If there is a flag error
	// or a configuration error, standard Annotee return codes (likely to overlap
	// with standard process return codes) will be used.
	//
	// This value has been chosen so as not to conflict with LogDog Butler runtime
	// error return code, allowing users to differentiate between Butler and
	// Annotee errors.
	runtimeErrorReturnCode = 251

	// defaultAnnotationInterval is the default annotation interval value.
	defaultAnnotationInterval = 30 * time.Second
)

type application struct {
	context.Context

	annotate           annotationMode
	resultPath         string
	jsonArgsPath       string
	butlerStreamServer string
	tee                bool
	printSummary       bool
	testingDir         string
	annotationInterval clockflag.Duration
	project            config.ProjectName
	nameBase           streamproto.StreamNameFlag
	prefix             streamproto.StreamNameFlag
	logdogHost         string

	bootstrap *bootstrap.Bootstrap
}

func (a *application) addToFlagSet(fs *flag.FlagSet) {
	fs.Var(&a.annotate, "annotate",
		"Annotation handling mode. Options are: "+annotationFlagEnum.Choices())
	fs.StringVar(&a.resultPath, "result-path", "",
		"If supplied, a JSON file describing the bootstrap result will be written here if the bootstrapped process "+
			"is successfully executed.")
	fs.StringVar(&a.jsonArgsPath, "json-args-path", "",
		"If specified, this is a JSON file containing the full command to run as an "+
			"array of strings.")
	fs.StringVar(&a.butlerStreamServer, "butler-stream-server", "",
		"The Butler stream server location. If empty, Annotee will check for Butler "+
			"bootstrapping and extract the stream server from that.")
	fs.BoolVar(&a.tee, "tee", true,
		"Tee the bootstrapped process' STDOUT/STDERR streams.")
	fs.BoolVar(&a.printSummary, "print-summary", true,
		"Print the annotation protobufs that were emitted at the end.")
	fs.StringVar(&a.testingDir, "testing-dir", "",
		"Rather than coupling to a Butler instance, output generated annotations "+
			"and streams to this directory.")
	fs.Var(&a.annotationInterval, "annotation-interval",
		"Buffer annotation updates for this amount of time. <=0 sends every update.")
	fs.Var(&a.project, "project", "The log prefix's project name (required).")
	fs.Var(&a.nameBase, "name-base", "Base stream name to prepend to generated names.")
	fs.Var(&a.prefix, "prefix", "The log stream prefix. If missing, one will be inferred from bootstrap.")
	fs.StringVar(&a.logdogHost, "logdog-host", "",
		"LogDog Coordinator host name. If supplied, log viewing links will be generated.")
}

func (a *application) loadJSONArgs() ([]string, error) {
	fd, err := os.Open(a.jsonArgsPath)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	dec := json.NewDecoder(fd)
	args := []string(nil)
	if err := dec.Decode(&args); err != nil {
		return nil, err
	}
	return args, nil
}

func (a *application) getStreamClient() (streamclient.Client, error) {
	if a.testingDir != "" {
		return newFilesystemClient(a.testingDir)
	}

	if a.butlerStreamServer != "" {
		return streamclient.New(a.butlerStreamServer)
	}

	// Check our bootstrap.
	if a.bootstrap != nil && a.bootstrap.Client != nil {
		return a.bootstrap.Client, nil
	}

	return nil, errors.New("unable to identify stream client")
}

func (a *application) maybeWriteResult(r *bootstrapResult.Result) error {
	if a.resultPath == "" {
		return nil
	}

	log.Fields{
		"path": a.resultPath,
	}.Debugf(a, "Writing bootstrap result.")
	return r.WriteJSON(a.resultPath)
}

func mainImpl(args []string) int {
	ctx := gologger.StdConfig.Use(context.Background())

	logFlags := log.Config{
		Level: log.Warning,
	}
	a := application{
		Context:            ctx,
		annotationInterval: clockflag.Duration(defaultAnnotationInterval),
	}

	// Determine bootstrapped process arguments.
	var err error
	a.bootstrap, err = bootstrap.Get()
	if err != nil {
		log.WithError(err).Warningf(a, "Could not get LogDog Butler bootstrap information.")
	}

	fs := &flag.FlagSet{}
	logFlags.AddFlags(fs)
	a.addToFlagSet(fs)
	if err := fs.Parse(args); err != nil {
		log.WithError(err).Errorf(a, "Failed to parse flags.")
		return configErrorReturnCode
	}
	a.Context = logFlags.Set(a.Context)

	client, err := a.getStreamClient()
	if err != nil {
		log.WithError(err).Errorf(a, "Failed to get stream client instance.")
		return configErrorReturnCode
	}

	prefix := types.StreamName(a.prefix)
	if prefix == "" && a.bootstrap != nil {
		prefix = a.bootstrap.Prefix
	}

	// Get the annotation project. This must be non-empty.
	if a.project == "" {
		a.project = a.bootstrap.Project
	}
	if err := a.project.Validate(); err != nil {
		log.WithError(err).Errorf(a, "Invalid project (-project).")
		return configErrorReturnCode
	}

	args = fs.Args()
	if a.jsonArgsPath != "" {
		if len(args) > 0 {
			log.Fields{
				"commandLineArgs": args,
				"jsonArgsPath":    a.jsonArgsPath,
			}.Errorf(a, "Cannot specify both JSON and command-line arguments.")
			return configErrorReturnCode
		}

		args, err = a.loadJSONArgs()
		if err != nil {
			log.Fields{
				log.ErrorKey:   err,
				"jsonArgsPath": a.jsonArgsPath,
			}.Errorf(a, "Failed to load JSON arguments.")
			return configErrorReturnCode
		}
	}
	if len(args) == 0 {
		log.Errorf(a, "No command-line arguments were supplied.")
		return configErrorReturnCode
	}

	// Translate "<=0" flag option to Processor's "0", indicating that every
	// update should be sent.
	if a.annotationInterval < 0 {
		a.annotationInterval = 0
	}

	// Initialize our link generator, if we can.
	e := executor.Executor{
		Options: annotee.Options{
			Base:                   types.StreamName(a.nameBase),
			Client:                 client,
			MetadataUpdateInterval: time.Duration(a.annotationInterval),
			CloseSteps:             true,
		},

		Annotate: executor.AnnotationMode(a.annotate),
		Stdin:    os.Stdin,
	}

	linkGen := &coordinatorLinkGenerator{
		host:    a.logdogHost,
		project: a.project,
		prefix:  prefix,
	}
	if linkGen.canGenerateLinks() {
		e.Options.LinkGenerator = linkGen
	}

	if a.tee {
		e.TeeStdout = os.Stdout
		e.TeeStderr = os.Stderr
	}
	if err := e.Run(a, args); err != nil {
		log.Fields{
			log.ErrorKey: err,
			"returnCode": e.ReturnCode(),
		}.Errorf(a, "Failed during execution.")
	}

	// Display a summary!
	if a.printSummary {
		// Unmarshal our Step data.
		var st milo.Step
		if err := proto.Unmarshal(e.Step(), &st); err == nil {
			fmt.Printf("=== Annotee: %q ===\n", st.Name)
			fmt.Println(proto.MarshalTextString(&st))
		} else {
			log.WithError(err).Warningf(a, "Failed to unmarshal end step data. Cannot show summary.")
		}
	}

	if !e.Executed() {
		return runtimeErrorReturnCode
	}

	br := bootstrapResult.Result{
		ReturnCode: e.ReturnCode(),
		Command:    args,
	}
	if err := a.maybeWriteResult(&br); err != nil {
		log.WithError(err).Warningf(a, "Failed to write bootstrap result.")
	}
	return e.ReturnCode()
}

func main() {
	os.Exit(mainImpl(os.Args[1:]))
}
