package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"reflect"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/sklog"

	"go.skia.org/infra/go/fileutil"

	"go.skia.org/infra/go/common"
)

// Command line flags.
var (
	templateFileName = flag.String("t", "", "Template file name.")
	configFileName   = flag.String("c", "", "Config file name")
	outputFileName   = flag.String("o", "", "Output file name.")
	action           = flag.String("action", "", "Actions: "+strings.Join(ACTIONS, "|"))
	extraVars        = common.NewMultiStringFlag("extra", nil, "Extra key value pair(s), separated by a colon, eg. \"key:value\"")
)

const (
	OP_INIT     = "init"
	OP_GENERATE = "gen"
)

var (
	ACTIONS = []string{OP_INIT, OP_GENERATE}
)

func main() {
	common.Init()

	tmpl, err := template.ParseFiles(*templateFileName)
	if err != nil {
		sklog.Fatalf("Error parsing template '%s'. Error:%s", *templateFileName, err)
	}

	extraVarsMap := map[string]string{}
	for _, pair := range *extraVars {
		split := strings.SplitN(pair, ":", 2)
		if len(split) != 2 {
			sklog.Fatalf("Invalid key/value pair for --extra: %q; should be \"key:value\"", pair)
		}
		extraVarsMap[split[0]] = split[1]
	}

	confFileNames := strings.Split(*configFileName, ",")
	switch *action {
	case OP_INIT:
		if err := initConfFile(tmpl, confFileNames); err != nil {
			sklog.Fatalf("Error: %s", err)
		}

	case OP_GENERATE:
		if err := generateOutput(tmpl, confFileNames, extraVarsMap, *outputFileName); err != nil {
			sklog.Fatalf("Error: %s", err)
		}
	}
}

func generateOutput(tmpl *template.Template, configFileNames []string, extraVarsMap map[string]string, outFile string) error {
	config, err := loadConfigFile(configFileNames...)
	if err != nil {
		return err
	}
	for k, v := range extraVarsMap {
		config[k] = v
	}

	sklog.Infof("Config: %s", spew.Sdump(config))

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, config); err != nil {
		sklog.Fatalf("Error: %s", err)
	}

	return ioutil.WriteFile(outFile, buf.Bytes(), 0644)
}

func parseConfigHelper(prefix string, confMap map[string]interface{}, ret map[string]string) {
	for k, v := range confMap {
		t := reflect.ValueOf(v).Type().String()
		val := ""
		if t == "string" {
			val = v.(string)
		} else if t == "bool" {
			if v.(bool) {
				val = "true"
			} else {
				val = "false"
			}
		} else if t == "map[string]interface {}" {
			parseConfigHelper(prefix+k+".", v.(map[string]interface{}), ret)
		} else {
			sklog.Warningf("Key %q has unsupported type %q", k, reflect.ValueOf(v).Type().String())
		}
		if val != "" {
			ret[prefix+k] = val
		}
	}
}

func loadConfigFile(configFileNames ...string) (map[string]string, error) {
	ret := map[string]string{}
	for _, configFile := range configFileNames {
		confMap := map[string]interface{}{}
		if err := config.ParseConfigFile(configFile, "-c", &confMap); err != nil {
			return nil, err
		}
		parseConfigHelper("", confMap, ret)
	}
	return ret, nil
}

var (
	lookup = map[parse.NodeType]string{
		parse.NodeText:   "NodeText",
		parse.NodeAction: "NodeAction",
		parse.NodeField:  "NodeField",
	}
)

func printNodes(prefix string, node parse.Node) {
	fmt.Printf("%s%s(%d)", prefix, lookup[node.Type()], node.Type())
	var children []parse.Node
	switch v := node.(type) {
	case *parse.ListNode:
		children = v.Nodes
	case *parse.ActionNode:
		for _, child := range v.Pipe.Decl {
			fmt.Printf("%s ", child.Ident)
		}
		for _, child := range v.Pipe.Cmds {
			children = append(children, child.Args...)
		}
	case *parse.FieldNode:
		for _, child := range v.Ident {
			fmt.Printf("%s ", child)
		}
	}

	fmt.Println()
	for _, child := range children {
		printNodes(prefix+"    ", child)
	}
}

func extractAndValidate(tmpl *template.Template) ([]string, error) {
	keys := []string{}
	printNodes("", tmpl.Root)
	return keys, nil
}

func initConfFile(tmpl *template.Template, configFileNames []string) error {
	configFile := configFileNames[len(configFileNames)-1]
	config := map[string]string{}
	var err error
	if fileutil.FileExists(configFile) {
		config, err = loadConfigFile(configFile)
		if err != nil {
			sklog.FmtErrorf("Unable to load existing config file %s. Error: %s", configFile, err)
		}
	}

	keys, err := extractAndValidate(tmpl)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if _, ok := config[key]; !ok {
			config[key] = ""
		}
	}

	jsonBytes, err := json.Marshal(config)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(configFile, jsonBytes, 0644)
}
