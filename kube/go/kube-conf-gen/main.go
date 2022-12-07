package main

import (
	"flag"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/kube/go/kube_conf_gen_lib"
)

func main() {
	var (
		// Define the flags and then parse them in common.Init.
		templateFileName = flag.String("t", "", "Template file name.")
		configFileNames  = common.NewMultiStringFlag("c", nil, "Config file name")
		outputFileName   = flag.String("o", "", "Output file name. Use \"-o _\" to write to stdout.")
		emptyQuotes      = flag.Bool("quote", false, "Replace config values that are empty strings with empty single quotes ('').")
		parseConf        = flag.Bool("parse_conf", true, "Convert config options to string.")
		extraVars        = common.NewMultiStringFlag("extra", nil, "Extra key value pair(s), separated by a colon, eg. \"key:value\"")
		strict           = flag.Bool("strict", false, "If true, error out for unsupported types, missing data, etc.")
	)
	common.Init()

	if *templateFileName == "" {
		sklog.Fatal("-t is required.")
	}
	if *outputFileName == "" {
		sklog.Fatal("-o is required.")
	}
	extraVarsMap := map[string]string{}
	for _, pair := range *extraVars {
		split := strings.SplitN(pair, ":", 2)
		if len(split) != 2 {
			sklog.Fatalf("Invalid key/value pair for --extra: %q; should be \"key:value\"", pair)
		}
		extraVarsMap[split[0]] = split[1]
	}

	// Assemble the config map.
	config, err := kube_conf_gen_lib.LoadConfigFiles(*parseConf, *strict, *emptyQuotes, *configFileNames...)
	if err != nil {
		sklog.Fatalf("Error loading config files: %s", err)
	}
	for k, v := range extraVarsMap {
		config[k] = v
	}

	// Generate the output.
	if err := kube_conf_gen_lib.GenerateOutputFromTemplateFile(*templateFileName, *strict, config, *outputFileName); err != nil {
		sklog.Fatal(err)
	}
}
