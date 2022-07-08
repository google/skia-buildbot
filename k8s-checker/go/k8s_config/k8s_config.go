package k8s_config

import (
	"strings"

	"go.skia.org/infra/go/skerr"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1beta1"
	"sigs.k8s.io/yaml"
)

const (
	// K8s config kinds.
	CronJobKind     = "CronJob"
	DeploymentKind  = "Deployment"
	StatefulSetKind = "StatefulSet"
)

// ParseK8sConfigFile parses the given config file contents and returns the
// configs it contains.
func ParseK8sConfigFile(contents []byte) ([]*apps.Deployment, []*apps.StatefulSet, []*batch.CronJob, error) {
	yamlDocs := strings.Split(string(contents), "---")
	deployments := []*apps.Deployment{}
	statefulSets := []*apps.StatefulSet{}
	cronJobs := []*batch.CronJob{}
	for _, yamlDoc := range yamlDocs {
		deployment := new(apps.Deployment)
		if err := yaml.Unmarshal([]byte(yamlDoc), deployment); err != nil {
			return nil, nil, nil, skerr.Wrapf(err, "failed to parse config file")
		}
		switch deployment.TypeMeta.Kind {
		case DeploymentKind:
			deployments = append(deployments, deployment)
		case StatefulSetKind:
			statefulSet := new(apps.StatefulSet)
			if err := yaml.Unmarshal([]byte(yamlDoc), statefulSet); err != nil {
				return nil, nil, nil, skerr.Wrapf(err, "failed to parse config file")
			}
			statefulSets = append(statefulSets, statefulSet)
		case CronJobKind:
			cronJob := new(batch.CronJob)
			if err := yaml.Unmarshal([]byte(yamlDoc), cronJob); err != nil {
				return nil, nil, nil, skerr.Wrapf(err, "failed to parse config file")
			}
			cronJobs = append(cronJobs, cronJob)
		}

	}
	return deployments, statefulSets, cronJobs, nil
}
