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
	"log"
	"regexp"
	"strings"

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
	// atComparison is used to chop up expressions at a comparison. Note that we
	// require a trailing space, which avoids matching the equals sign inside
	// a label, such as {app="foo"}.
	atComparison = regexp.MustCompile(`[<>=!]+\s`)
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

// equationFromExpr returns a bool that is true if the equation should be ignored.
func equationFromExpr(expr string) (string, bool) {
	if expr == "" {
		return "", false
	}
	// Ignore computed metrics, which by convention have a ":".
	if strings.Contains(expr, ":") {
		return "", true
	}
	parts := atComparison.Split(Reverse(expr), -1)
	// Ignore multipart relations, e.g. "a < b and b > c".
	if len(parts) != 2 {
		return "", true
	}
	return strings.TrimSpace(Reverse(parts[1])), false
}

func main() {
	flag.Parse()
	b, err := ioutil.ReadFile(*input)
	if err != nil {
		log.Fatalf("Failed to read %q: %s", *input, err)
	}
	var alerts Alerts
	if err := yaml.Unmarshal(b, &alerts); err != nil {
		log.Fatalf("Failed to parse %q: %s", *input, err)
	}

	absent := Alerts{
		Groups: []Group{},
	}

	for _, g := range alerts.Groups {
		rules := []Rule{}
		for _, rule := range g.Rules {
			equation, ignore := equationFromExpr(rule.Expr)
			if ignore {
				continue
			}
			if equation == "" {
				log.Fatalf("Failed to extract an eqation for %q", rule.Alert)
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
		log.Fatalf("Failed to marshall as YAML: %s", err)
	}
	if err := ioutil.WriteFile(*output, b, 0664); err != nil {
		log.Fatalf("Failed to write %q: %s", *output, err)
	}
}
