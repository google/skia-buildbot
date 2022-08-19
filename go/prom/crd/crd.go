// Package crd handles Managed Prometheus Custom Resource Definitions.
package crd

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/prom"
)

const notInClustersAnnotationKey = "not_in_clusters"

// Rules Custom Resource representation.
//
// In theory we should be able to import this from the Managed Prometheus repo, but
// they don't currently provide a separate repo with just the json annotated structs.
type Rules struct {
	Version  string   `yaml:"apiVersion"`
	Kind     string   `yaml:"kind"`
	MetaData MetaData `yaml:"metadata"`
	Spec     Spec     `yaml:"spec"`
}

// MetaData for the Rules CRD.
type MetaData struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

// Spec for parsing the yaml format of Prometheus alerts.
type Spec struct {
	Groups []Group `yaml:"groups"`
}

// Group of Rules.
type Group struct {
	Name     string `yaml:"name"`
	Interval string `yaml:"interval"`
	Rules    []Rule `yaml:"rules"`
}

// Rule is a single Prometheus Alert.
type Rule struct {
	Alert       string            `yaml:"alert"`
	Expr        string            `yaml:"expr"`
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
}

// Skip returns true if the alert should not be applied to the given cluster.
func (r Rule) Skip(cluster string) bool {
	if skipString, ok := r.Annotations[notInClustersAnnotationKey]; ok {
		for _, skipCluster := range strings.Split(skipString, ",") {
			if cluster == strings.TrimSpace(skipCluster) {
				return true
			}
		}
	}
	return false
}

// AddAbsentRules adds an `absent()` alert for each Rule, where possible.
func (r *Rules) AddAbsentRules(cluster string) {
	absentGroups := []Group{}
	for _, g := range r.Spec.Groups {
		rules := []Rule{}
		for _, rule := range g.Rules {
			if rule.Skip(cluster) {
				continue
			}
			equation, ignore := prom.EquationFromExpr(rule.Expr)
			if ignore {
				continue
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
					"equation":    equation,
					"description": fmt.Sprintf("There is no data for the Alert: %q", rule.Alert),
				},
			})
		}
		absentGroups = append(absentGroups, Group{
			Name:     fmt.Sprintf("absent-%s", g.Name),
			Interval: g.Interval,
			Rules:    rules,
		})
	}
	r.Spec.Groups = append(r.Spec.Groups, absentGroups...)
}
