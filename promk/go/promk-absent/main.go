// An application to create a new set of alerts from an existing set of alerts.
//
// The new alerts detect if no data is present for the associated alert.
//
// Presumes that all expressions are written in the form of:
//
//    equation [<>!=]+ (some constant)
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	yaml "gopkg.in/yaml.v2"
)

// Alerts - Struct for parsing the yaml format of Prometheus alerts.
type Alerts struct {
	Groups []Group `yaml:"groups"`
}

type Group struct {
	Name  string `yaml:"name"`
	Rules []Rule `yaml:"rules"`
}

type Rule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// flags
var (
	input  = flag.String("input", "", "Name of file to read.")
	output = flag.String("output", "", "Name of file to write.")
)

var (
	atComparison = regexp.MustCompile("[<>=!]+")
)

// Reverse a string.
//
// https://github.com/golang/example/blob/master/stringutil/reverse.go
func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func equationFromExpr(expr string) string {
	if expr == "" {
		return ""
	}
	return strings.TrimSpace(Reverse(atComparison.Split(Reverse(expr), 2)[1]))
}

func main() {
	common.Init()
	b, err := ioutil.ReadFile(*input)
	if err != nil {
		sklog.Fatal(err)
	}
	var alerts Alerts
	if err := yaml.Unmarshal(b, &alerts); err != nil {
		sklog.Fatal(err)
	}

	absent := Alerts{
		Groups: []Group{},
	}

	for _, g := range alerts.Groups {
		rules := []Rule{}
		for _, rule := range g.Rules {
			equation := equationFromExpr(rule.Expr)
			if equation == "" {
				sklog.Fatalf("Failed to extract an eqation for %q", rule.Alert)
			}
			rules = append(rules, Rule{
				Alert: "Absent",
				Expr:  fmt.Sprintf("absent(%s)", equation),
				Labels: map[string]string{
					"category": "infra",
					"severify": "critical",
				},
				Annotations: map[string]string{
					"abbr":        rule.Alert,
					"description": fmt.Sprintf("There is no data for the Alert: %q", rule.Alert),
				},
			})
		}
		absent.Groups = append(absent.Groups, Group{
			Name:  g.Name,
			Rules: rules,
		})
	}

	b, err = yaml.Marshal(absent)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ioutil.WriteFile(*output, b, 0664); err != nil {
		sklog.Fatal(err)
	}
}
