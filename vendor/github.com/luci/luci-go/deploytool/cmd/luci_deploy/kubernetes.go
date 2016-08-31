// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"sort"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/deploytool/api/deploy"

	"golang.org/x/net/context"
	"gopkg.in/yaml.v2"
)

var errKubeResourceNotFound = errors.New("resource not found")

// kubeDeployment defines a [v1beta1.Deployment].
type kubeDeployment struct {
	Kind       string                `yaml:"kind"`
	APIVersion string                `yaml:"apiVersion"`
	Metadata   *kubeObjectMeta       `yaml:"metadata,omitempty"`
	Spec       *kubeDeploymentSpec   `yaml:"spec,omitempty"`
	Status     *kubeDeploymentStatus `yaml:"status,omitempty"`
}

// kubeDeploymentSpec defines a [v1beta1.DeploymentSpec].
type kubeDeploymentSpec struct {
	Replicas int                  `yaml:"replicas,omitempty"`
	Selector *kubeLabelSelector   `yaml:"selector,omitempty"`
	Template *kubePodTemplateSpec `yaml:"template"`

	MinReadySeconds      int  `yaml:"minReadySeconds,omitempty"`
	RevisionHistoryLimit int  `yaml:"revisionHistoryLimit,omitempty"`
	Paused               bool `yaml:"paused,omitempty"`
}

// kubeDeploymentSpec defines a [v1beta1.DeploymentStatus].
type kubeDeploymentStatus struct {
	ObservedGeneration  int `yaml:"observedGeneration"`
	Replicas            int `yaml:"replicas"`
	UpdatedReplicas     int `yaml:"updatedReplicas"`
	AvailableRepicas    int `yaml:"availableReplicas"`
	UnavailableReplicas int `yaml:"unavailableReplicas"`
}

// kubePodTemplateSpec defines a [v1.PodTemplateSpec].
type kubePodTemplateSpec struct {
	Metadata *kubeObjectMeta `yaml:"metadata,omitempty"`
	Spec     *kubePodSpec    `yaml:"spec,omitempty"`
}

// kubePodSpec defines a [v1.PodSpec].
type kubePodSpec struct {
	Containers    []*kubeContainer `yaml:"containers"`
	RestartPolicy string           `yaml:"restartPolicy,omitempty"`

	TerminationGracePeriodSeconds int `yaml:"terminationGracePeriodSeconds,omitempty"`
	ActiveDeadlineSeconds         int `yaml:"activeDeadlineSeconds,omitempty"`
}

// kubeContainer defines a [v1.Container].
type kubeContainer struct {
	Name       string   `yaml:"name"`
	Image      string   `yaml:"image,omitempty"`
	Command    []string `yaml:"command,omitempty"`
	Args       []string `yaml:"args,omitempty"`
	WorkingDir string   `yaml:"workingDir,omitempty"`

	Ports     []*kubeContainerPort      `yaml:"ports,omitempty"`
	Env       []*kubeEnvVar             `yaml:"env,omitempty"`
	Resources *kubeResourceRequirements `yaml:"resources,omitempty"`

	LivenessProbe  *kubeProbe `yaml:"livenessProbe,omitempty"`
	ReadinessProbe *kubeProbe `yaml:"readinessProbe,omitempty"`

	Lifecycle *kubeLifecycle `yaml:"lifecycle,omitempty"`
}

// kubeContainerPort defines a [v1.ContainerPort].
type kubeContainerPort struct {
	Name          string `yaml:"name,omitempty"`
	HostPort      int    `yaml:"hostPort,omitempty"`
	ContainerPort int    `yaml:"containerPort"`
	Protocol      string `yaml:"protocol,omitempty"`
	HostIP        string `yaml:"hostIP,omitempty"`
}

// kubeEnvVar defines a [v1.EnvVar].
type kubeEnvVar struct {
	Name      string            `yaml:"name"`
	Value     string            `yaml:"name,omitempty"`
	ValueFrom *kubeEnvVarSource `yaml:"valueFrom,omitempty"`
}

// kubeEnvVarSource represents a [v1.EnvVarSource].
type kubeEnvVarSource struct {
	FieldRef        *kubeObjectFieldSelector
	ConfigMapKeyRef *kubeConfigMapKeySelector `yaml:"configMapKeyRef,omitempty"`
	SecretKeyRef    *kubeSecretKeySelector    `yaml:"secretKeyRef,omitempty"`
}

// kubeLabelSelector represents a [v1beta1.LabelSelector].
type kubeLabelSelector struct {
	MatchLabels      map[string]interface{}        `yaml:"matchLabels,omitempty"`
	MatchExpressions *kubeLabelSelectorRequirement `yaml:"matchExpressions,omitempty"`
}

// kubeLabelSelectorRequirement represets a [v1beta1.LabelSelectorRequirement].
type kubeLabelSelectorRequirement struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values,omitempty"`
}

// kubeObjectFieldSelector defines a [v1.ObjectFieldSelector].
type kubeObjectFieldSelector struct {
	APIVersion string `yaml:"apiVersion,omitempty"`
	FieldPath  string `yaml:"fieldPath"`
}

// kubeConfigMapKeySelector defines a [v1.ConfigMapKeySelector].
type kubeConfigMapKeySelector struct {
	Name string `yaml:"name,omitempty"`
	Key  string `yaml:"key"`
}

// kubeSecretKeySelector defines a [v1.SecretKeySelector].
type kubeSecretKeySelector struct {
	Name string `yaml:"name,omitempty"`
	Key  string `yaml:"key"`
}

// kubeResourceRequirements defines a [v1.ResourceRequirements].
type kubeResourceRequirements struct {
	Limits   map[string]interface{} `yaml:"limits,omitempty"`
	Requests map[string]interface{} `yaml:"requests,omitempty"`
}

// kubeHTTPGetAction defines a [v1.HTTPGetAction].
type kubeHTTPGetAction struct {
	Path   string `yaml:"path,omitempty"`
	Port   int    `yaml:"port"`
	Host   string `yaml:"host,omitempty"`
	Scheme string `yaml:"scheme,omitempty"`

	HTTPHeaders []*kubeHTTPHeader `yaml:"httpHeaders,omitempty"`
}

// kubeHTTPHeader defines a [v1.HTTPHeader].
type kubeHTTPHeader struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value,omitempty"`
}

// kubeProbe defines a [v1.Probe]
type kubeProbe struct {
	// Exec defines a [v1.ExecAction].
	Exec    *kubeExecAction
	HTTPGet *kubeHTTPGetAction `yaml:"httpGet,omitempty"`

	InitialDelaySeconds int `yaml:"initialDelaySeconds,omitempty"`
	TimeoutSeconds      int `yaml:"timeoutSeconds,omitempty"`
	PeriodSeconds       int `yaml:"periodSeconds,omitempty"`
	SuccessThreshold    int `yaml:"successThreshold,omitempty"`
	FailureThreshold    int `yaml:"failureThreshold,omitempty"`
}

// kubeExecAction defines a [v1.ExecAction].
type kubeExecAction struct {
	Command []string `yaml:"command"`
}

// kubeLifecycle defines a [v1.Lifecycle].
type kubeLifecycle struct {
	PostStart *kubeHandler `yaml:"postStart,omitempty"`
	PreStop   *kubeHandler `yaml:"preStop,omitempty"`
}

// kubeHandler defines a [v1.Handler].
type kubeHandler struct {
	Exec    *kubeExecAction    `yaml:"exec,omitempty"`
	HTTPGet *kubeHTTPGetAction `yaml:"httpGet,omitempty"`
}

// kubeObjectMeta defines a [v1.ObjectMeta].
type kubeObjectMeta struct {
	Name         string `yaml:"name,omitempty"`
	GenerateName string `yaml:"generateName,omitempty"`
	Namespace    string `yaml:"namespace,omitempty"`

	SelfLink                   string `yaml:"selfLink,omitempty"`
	UID                        string `yaml:"uid,omitempty"`
	ResourceVersion            string `yaml:"resourceVersion,omitempty"`
	Generation                 string `yaml:"generation,omitempty"`
	CreationTimestamp          string `yaml:"creationTimestamp,omitempty"`
	DeletionTimestamp          string `yaml:"deletionTimestamp,omitempty"`
	DeletionGracePeriodSeconds int    `yaml:"deletionGracePeriodSeconds,omitempty"`

	// Labels defines a series of [any] labels.
	Labels map[string]interface{} `yaml:"labels,omitempty"`
	// Annotations defines a series of [any] annotations.
	Annotations map[string]interface{} `yaml:"annotations,omitempty"`
}

func (meta *kubeObjectMeta) addLabel(key string, value interface{}) {
	if meta.Labels == nil {
		meta.Labels = make(map[string]interface{})
	}
	meta.Labels[key] = value
}

func (meta *kubeObjectMeta) addAnnotation(key string, value interface{}) {
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]interface{})
	}
	meta.Annotations[key] = value
}

func kubeBuildDeploymentYAML(pb *layoutDeploymentGKEPodBinding, name string,
	imageMap map[string]string) *kubeDeployment {
	kp := pb.pod.KubePod
	dep := kubeDeployment{
		Kind:       "Deployment",
		APIVersion: "extensions/v1beta1",
		Metadata: &kubeObjectMeta{
			Name:        name,
			Labels:      make(map[string]interface{}),
			Annotations: make(map[string]interface{}),
		},
		Spec: &kubeDeploymentSpec{
			Replicas: int(pb.Replicas),
			Template: &kubePodTemplateSpec{
				Metadata: &kubeObjectMeta{},
				Spec: &kubePodSpec{
					Containers:                    make([]*kubeContainer, len(kp.Container)),
					RestartPolicy:                 kp.RestartPolicy.KubeString(),
					TerminationGracePeriodSeconds: int(kp.TerminationGracePeriod.Duration().Seconds()),
					ActiveDeadlineSeconds:         int(kp.ActiveDeadline.Duration().Seconds()),
				},
			},
			MinReadySeconds: int(kp.MinReady.Duration().Seconds()),
		},
	}
	tmpl := dep.Spec.Template

	// Pod Template Metadata
	for k, v := range kp.Labels {
		tmpl.Metadata.Labels[k] = v
	}

	// Pod Template Containers
	for i, kc := range kp.Container {
		cont := kubeContainer{
			Name:       kc.Name,
			Image:      imageMap[kc.Name],
			Command:    kc.Command,
			Args:       kc.Args,
			WorkingDir: kc.WorkingDir,
		}

		// Ports
		if len(kc.Ports) > 0 {
			cont.Ports = make([]*kubeContainerPort, len(kc.Ports))
			for i, port := range kc.Ports {
				cont.Ports[i] = &kubeContainerPort{
					Name:          port.Name,
					ContainerPort: int(port.ContainerPort),
				}
			}
		}

		// Environment
		if len(kc.Env) > 0 {
			cont.Env = make([]*kubeEnvVar, 0, len(kc.Env))
			for k, v := range kc.Env {
				cont.Env = append(cont.Env, &kubeEnvVar{
					Name:  k,
					Value: v,
				})
			}
			sort.Sort(sortableEnvVarSlice(cont.Env))
		}

		// Resources
		var res kubeResourceRequirements
		for _, r := range []struct {
			r *deploy.KubernetesPod_Container_Resources
			p *map[string]interface{}
		}{
			{kc.Limits, &res.Limits},
			{kc.Requested, &res.Requests},
		} {
			if r.r == nil {
				continue
			}

			m := make(map[string]interface{}, 2)
			if cpu := r.r.Cpu; cpu > 0 {
				m["cpu"] = cpu
			}
			if mem := r.r.Memory; mem != nil {
				m["memory"] = fmt.Sprintf("%d%s", mem.Amount, mem.Unit.KubeSuffix())
			}
			if len(m) > 0 {
				// We have at least one resource value, so assign.
				*r.p = m
				cont.Resources = &res
			}
		}

		// Probes
		for _, p := range []struct {
			v *deploy.KubernetesPod_Container_Probe
			p **kubeProbe
		}{
			{kc.LivenessProbe, &cont.LivenessProbe},
			{kc.ReadinessProbe, &cont.ReadinessProbe},
		} {
			if p.v == nil {
				continue
			}

			probe := kubeProbe{
				InitialDelaySeconds: int(p.v.InitialDelay.Duration().Seconds()),
				TimeoutSeconds:      int(p.v.Timeout.Duration().Seconds()),
				PeriodSeconds:       int(p.v.Period.Duration().Seconds()),
				SuccessThreshold:    int(p.v.SuccessThreshold),
				FailureThreshold:    int(p.v.FailureThreshold),
			}
			if exec := p.v.Exec; len(exec) > 0 {
				probe.Exec = &kubeExecAction{
					Command: exec,
				}
			}
			if hg := p.v.HttpGet; hg != nil {
				probe.HTTPGet = kubeMakeHTTPGetAction(hg)
			}
			*p.p = &probe
		}

		// Handlers
		var lc kubeLifecycle
		for _, h := range []struct {
			v *deploy.KubernetesPod_Container_Handler
			p **kubeHandler
		}{
			{kc.PostStart, &lc.PostStart},
			{kc.PreStop, &lc.PreStop},
		} {
			if h.v == nil {
				continue
			}

			var handler kubeHandler
			if exec := h.v.ExecCommand; len(exec) > 0 {
				handler.Exec = &kubeExecAction{
					Command: exec,
				}
			}
			if hg := h.v.HttpGet; hg != nil {
				handler.HTTPGet = kubeMakeHTTPGetAction(hg)
			}

			// We have an entry here, so assign.
			*h.p = &handler
			cont.Lifecycle = &lc
		}

		tmpl.Spec.Containers[i] = &cont
	}

	return &dep
}

func kubeMakeHTTPGetAction(hg *deploy.KubernetesPod_Container_HttpGet) *kubeHTTPGetAction {
	act := kubeHTTPGetAction{
		Path:   hg.Path,
		Port:   int(hg.Port),
		Host:   hg.Host,
		Scheme: hg.Scheme,
	}
	if len(hg.Headers) > 0 {
		act.HTTPHeaders = make([]*kubeHTTPHeader, len(hg.Headers))
		for i, hdr := range hg.Headers {
			act.HTTPHeaders[i] = &kubeHTTPHeader{
				Key:   hdr.Name,
				Value: hdr.Value,
			}
		}
	}
	return &act
}

type sortableEnvVarSlice []*kubeEnvVar

func (s sortableEnvVarSlice) Len() int           { return len(s) }
func (s sortableEnvVarSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s sortableEnvVarSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// kubeTool wraps the "kubectl" tool.
type kubeTool struct {
	exe string
	ctx string
}

func (t *kubeTool) hasContext(c context.Context) (bool, error) {
	x := execute(t.exe, "config", "view",
		"--output", fmt.Sprintf(`jsonpath='{.users[?(@.name == %q)].name}'`, t.ctx))
	if err := x.check(c); err != nil {
		return false, err
	}
	return (x.stdout.String() != "''"), nil
}

func (t *kubeTool) exec(commands ...string) *workExecutor {
	args := make([]string, 0, 2+len(commands))
	args = append(args, "--context", t.ctx)
	args = append(args, commands...)
	return execute(t.exe, args...)
}

func (t *kubeTool) getResource(c context.Context, resource string, obj interface{}) error {
	x := t.exec("get", resource, "-o", "yaml")
	switch rv, err := x.run(c); {
	case err != nil:
		return err

	case rv != 0:
		return errKubeResourceNotFound

	default:
		if err := yaml.Unmarshal(x.stdout.Bytes(), obj); err != nil {
			return errors.Annotate(err).Reason("failed to unmarshal YAML %(type)T").D("type", obj).Err()
		}
		return nil
	}
}
