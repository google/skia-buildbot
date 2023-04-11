package k8s_config

import (
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	apps "k8s.io/api/apps/v1"
	batch "k8s.io/api/batch/v1beta1"
	core "k8s.io/api/core/v1"
	policy "k8s.io/api/policy/v1beta1"
	rbac "k8s.io/api/rbac/v1"
	"sigs.k8s.io/yaml"
)

const (
	// K8s config kinds.
	ClusterRoleKind        = "ClusterRole"
	ClusterRoleBindingKind = "ClusterRoleBinding"
	CronJobKind            = "CronJob"
	DaemonSetKind          = "DaemonSet"
	DeploymentKind         = "Deployment"
	NamespaceKind          = "Namespace"
	PodSecurityPolicyKind  = "PodSecurityPolicy"
	RoleBindingKind        = "RoleBinding"
	ServiceKind            = "Service"
	ServiceAccountKind     = "ServiceAccount"
	StatefulSetKind        = "StatefulSet"

	// Unsupported kinds.
	BackendConfigKind        = "BackendConfig"
	ClusterPodMonitoringKind = "ClusterPodMonitoring"
	ClusterRulesKind         = "ClusterRules"
	ConfigMapKind            = "ConfigMap"
	IngressKind              = "Ingress"
	OperatorConfigKind       = "OperatorConfig"
	PodDisruptionBudgetKind  = "PodDisruptionBudget"
	StorageClassKind         = "StorageClass"
)

type K8sConfigFile struct {
	ClusterRole        []*rbac.ClusterRole
	ClusterRoleBinding []*rbac.ClusterRoleBinding
	CronJob            []*batch.CronJob
	DaemonSet          []*apps.DaemonSet
	Deployment         []*apps.Deployment
	Namespace          []*core.Namespace
	PodSecurityPolicy  []*policy.PodSecurityPolicy
	RoleBinding        []*rbac.RoleBinding
	Service            []*core.Service
	ServiceAccount     []*core.ServiceAccount
	StatefulSet        []*apps.StatefulSet
}

// ParseK8sConfigFile parses the given config file contents and returns the
// configs it contains.
func ParseK8sConfigFile(contents []byte) (*K8sConfigFile, error) {
	yamlDocs := strings.Split(string(contents), "---")
	rv := new(K8sConfigFile)
	for _, yamlDoc := range yamlDocs {
		if err := parseYamlDoc(yamlDoc, rv); err != nil {
			return nil, skerr.Wrap(err)
		}
	}
	return rv, nil
}

// ParseYamlDoc parses a single YAML document and returns the configs it
// contains.
func ParseYamlDoc(yamlDoc string) (*K8sConfigFile, error) {
	rv := new(K8sConfigFile)
	if err := parseYamlDoc(yamlDoc, rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

func parseYamlDoc(yamlDoc string, rv *K8sConfigFile) error {
	// Ignore empty documents.
	if strings.TrimSpace(yamlDoc) == "" {
		return nil
	}
	// Arbitrarily parse the document as a parsed. This makes it easy to
	// reference the TypeMeta.Kind, so that we can parse the correct type.
	parsed := new(apps.Deployment)
	if err := yaml.Unmarshal([]byte(yamlDoc), parsed); err != nil {
		return skerr.Wrapf(err, "failed to parse config file")
	}
	switch parsed.TypeMeta.Kind {
	case ClusterRoleKind:
		v := new(rbac.ClusterRole)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ClusterRole = append(rv.ClusterRole, v)
	case ClusterRoleBindingKind:
		v := new(rbac.ClusterRoleBinding)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ClusterRoleBinding = append(rv.ClusterRoleBinding, v)
	case CronJobKind:
		v := new(batch.CronJob)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.CronJob = append(rv.CronJob, v)
	case DaemonSetKind:
		v := new(apps.DaemonSet)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.DaemonSet = append(rv.DaemonSet, v)
	case DeploymentKind:
		v := new(apps.Deployment)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Deployment = append(rv.Deployment, v)
	case NamespaceKind:
		v := new(core.Namespace)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Namespace = append(rv.Namespace, v)
	case PodSecurityPolicyKind:
		v := new(policy.PodSecurityPolicy)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.PodSecurityPolicy = append(rv.PodSecurityPolicy, v)
	case RoleBindingKind:
		v := new(rbac.RoleBinding)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.RoleBinding = append(rv.RoleBinding, v)
	case ServiceKind:
		v := new(core.Service)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Service = append(rv.Service, v)
	case ServiceAccountKind:
		v := new(core.ServiceAccount)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ServiceAccount = append(rv.ServiceAccount, v)
	case StatefulSetKind:
		v := new(apps.StatefulSet)
		if err := yaml.Unmarshal([]byte(yamlDoc), v); err != nil {
			return skerr.Wrapf(err, "failed to parse config file")
		}
		rv.StatefulSet = append(rv.StatefulSet, v)
	case BackendConfigKind, ClusterPodMonitoringKind, ClusterRulesKind, ConfigMapKind, IngressKind, OperatorConfigKind, PodDisruptionBudgetKind, StorageClassKind:
		// We ignore these Kinds because we don't do anything with them at
		// this time.
	default:
		// Log a warning to indicate that we don't know about this Kind.
		sklog.Warningf("Unknown Kind %q", parsed.TypeMeta.Kind)
	}
	return nil
}
