package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"regexp"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	yaml "gopkg.in/yaml.v2"
)

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
	input = flag.String("input", "", "Name of file to read.")
)

func Reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < len(r)/2; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

func main() {
	common.Init()
	b, err := ioutil.ReadFile(*input)
	if err != nil {
		sklog.Fatal(err)
	}
	var alerts Alerts
	yaml.Unmarshal(b, &alerts)

	absent := Alerts{
		Groups: []Group{},
	}

	atComparison := regexp.MustCompile("[<>=!]+")

	for _, g := range alerts.Groups {
		rules := []Rule{}
		for _, rule := range g.Rules {
			equation := Reverse(atComparison.Split(Reverse(rule.Expr), 2)[1])
			fmt.Printf("%q -> %q\n", rule.Expr, equation)
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
	fmt.Println(string(b))
}
