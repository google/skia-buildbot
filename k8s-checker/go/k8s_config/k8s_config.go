package k8s_config

import (
	"bytes"

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

type ByteRange struct {
	Start int
	End   int
}

// SplitYAMLDocs splits the given YAML file contents into its component YAML
// documents. Returns the documents and their line ranges within the file.
func SplitYAMLDocs(contents []byte) ([][]byte, []*ByteRange) {
	var yamlDocs [][]byte
	var byteRanges []*ByteRange
	docStart := 0
	sepChars := 0
	for idx, b := range contents {
		if b == '-' {
			sepChars++
		} else {
			sepChars = 0
		}
		if sepChars == 3 {
			docEnd := idx - 2
			doc := bytes.TrimSpace(contents[docStart:docEnd])
			if len(doc) > 0 {
				yamlDocs = append(yamlDocs, doc)
				byteRanges = append(byteRanges, &ByteRange{
					Start: docStart,
					End:   docEnd,
				})
			}
			docStart = idx + 1
		}
	}
	doc := bytes.TrimSpace(contents[docStart:])
	if len(doc) > 0 {
		yamlDocs = append(yamlDocs, doc)
		byteRanges = append(byteRanges, &ByteRange{
			Start: docStart,
			End:   len(contents),
		})
	}
	return yamlDocs, byteRanges
}

// ParseK8sConfigFile parses the given config file contents and returns the
// configs it contains.
func ParseK8sConfigFile(contents []byte) (*K8sConfigFile, map[interface{}]*ByteRange, error) {
	yamlDocs, lineRanges := SplitYAMLDocs(contents)
	rv := new(K8sConfigFile)
	byteRanges := make(map[interface{}]*ByteRange, len(yamlDocs))
	for idx, yamlDoc := range yamlDocs {
		obj, err := parseYamlDoc(yamlDoc, rv)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if obj != nil {
			byteRanges[obj] = lineRanges[idx]
		}
	}
	return rv, byteRanges, nil
}

// ParseYamlDoc parses a single YAML document and returns the configs it
// contains.
func ParseYamlDoc(yamlDoc []byte) (*K8sConfigFile, error) {
	rv := new(K8sConfigFile)
	if _, err := parseYamlDoc(yamlDoc, rv); err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

func parseYamlDoc(yamlDoc []byte, rv *K8sConfigFile) (interface{}, error) {
	// Ignore empty documents.
	if len(bytes.TrimSpace(yamlDoc)) == 0 {
		return nil, nil
	}
	// Arbitrarily parse the document as a Deployment. This makes it easy to
	// reference the TypeMeta.Kind, so that we can parse the correct type.
	parsed := new(apps.Deployment)
	if err := yaml.Unmarshal(yamlDoc, parsed); err != nil {
		return nil, skerr.Wrapf(err, "failed to parse config file")
	}
	switch parsed.TypeMeta.Kind {
	case ClusterRoleKind:
		v := new(rbac.ClusterRole)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ClusterRole = append(rv.ClusterRole, v)
		return v, nil
	case ClusterRoleBindingKind:
		v := new(rbac.ClusterRoleBinding)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ClusterRoleBinding = append(rv.ClusterRoleBinding, v)
		return v, nil
	case CronJobKind:
		v := new(batch.CronJob)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.CronJob = append(rv.CronJob, v)
		return v, nil
	case DaemonSetKind:
		v := new(apps.DaemonSet)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.DaemonSet = append(rv.DaemonSet, v)
		return v, nil
	case DeploymentKind:
		v := new(apps.Deployment)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Deployment = append(rv.Deployment, v)
		return v, nil
	case NamespaceKind:
		v := new(core.Namespace)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Namespace = append(rv.Namespace, v)
		return v, nil
	case PodSecurityPolicyKind:
		v := new(policy.PodSecurityPolicy)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.PodSecurityPolicy = append(rv.PodSecurityPolicy, v)
		return v, nil
	case RoleBindingKind:
		v := new(rbac.RoleBinding)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.RoleBinding = append(rv.RoleBinding, v)
		return v, nil
	case ServiceKind:
		v := new(core.Service)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.Service = append(rv.Service, v)
		return v, nil
	case ServiceAccountKind:
		v := new(core.ServiceAccount)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.ServiceAccount = append(rv.ServiceAccount, v)
		return v, nil
	case StatefulSetKind:
		v := new(apps.StatefulSet)
		if err := yaml.Unmarshal(yamlDoc, v); err != nil {
			return nil, skerr.Wrapf(err, "failed to parse config file")
		}
		rv.StatefulSet = append(rv.StatefulSet, v)
		return v, nil
	case BackendConfigKind, ClusterPodMonitoringKind, ClusterRulesKind, ConfigMapKind, IngressKind, OperatorConfigKind, PodDisruptionBudgetKind, StorageClassKind:
		// We ignore these Kinds because we don't do anything with them at
		// this time.
		return nil, nil
	default:
		// Log a warning to indicate that we don't know about this Kind.
		sklog.Warningf("Unknown Kind %q", parsed.TypeMeta.Kind)
		return nil, nil
	}
}
