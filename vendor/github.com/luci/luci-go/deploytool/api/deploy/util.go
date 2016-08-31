// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package deploy

import (
	"fmt"
)

// AppYAMLString returns the "app.yaml" string for this option.
func (v AppEngineModule_Handler_LoginOption) AppYAMLString() string {
	switch v {
	case AppEngineModule_Handler_LOGIN_OPTIONAL:
		return "optional"
	case AppEngineModule_Handler_LOGIN_REQUIRED:
		return "required"
	case AppEngineModule_Handler_LOGIN_ADMIN:
		return "admin"
	default:
		panic(fmt.Errorf("unknown handler login option %q", v))
	}
}

// AppYAMLString returns the "app.yaml" string for this option.
func (v AppEngineModule_Handler_SecureOption) AppYAMLString() string {
	switch v {
	case AppEngineModule_Handler_SECURE_OPTIONAL:
		return "optional"
	case AppEngineModule_Handler_SECURE_NEVER:
		return "never"
	case AppEngineModule_Handler_SECURE_ALWAYS:
		return "always"
	default:
		panic(fmt.Errorf("unknown handler secure option %q", v))
	}
}

// AppYAMLString returns the "app.yaml" string for this option.
func (v AppEngineResources_Index_Direction) AppYAMLString() string {
	switch v {
	case AppEngineResources_Index_ASCENDING:
		return "asc"
	case AppEngineResources_Index_DESCENDING:
		return "desc"
	default:
		panic(fmt.Errorf("unknown index direction %q", v))
	}
}

// KubeString returns the Kubernetes "restartPolicy" field string for the
// enumeration value.
func (v KubernetesPod_RestartPolicy) KubeString() string {
	switch v {
	case KubernetesPod_RESTART_ALWAYS:
		return "Always"
	case KubernetesPod_RESTART_ON_FAILURE:
		return "OnFailure"
	case KubernetesPod_RESTART_NEVER:
		return "Never"
	default:
		panic(fmt.Errorf("unknown restart policy (%v)", v))
	}
}

// KubeSuffix returns the Kubernetes configuration unit suffix for a given
// resource unit.
func (v KubernetesPod_Container_Resources_Unit) KubeSuffix() string {
	switch v {
	case KubernetesPod_Container_Resources_BYTE:
		return ""

	case KubernetesPod_Container_Resources_KILOBYTE:
		return "K"
	case KubernetesPod_Container_Resources_MEGABYTE:
		return "M"
	case KubernetesPod_Container_Resources_GIGABYTE:
		return "G"
	case KubernetesPod_Container_Resources_TERABYTE:
		return "T"
	case KubernetesPod_Container_Resources_PETABYTE:
		return "P"
	case KubernetesPod_Container_Resources_EXABYTE:
		return "E"

	case KubernetesPod_Container_Resources_KIBIBYTE:
		return "Ki"
	case KubernetesPod_Container_Resources_MEBIBYTE:
		return "Mi"
	case KubernetesPod_Container_Resources_GIBIBYTE:
		return "Gi"
	case KubernetesPod_Container_Resources_TEBIBYTE:
		return "Ti"
	case KubernetesPod_Container_Resources_PEBIBYTE:
		return "Pi"
	case KubernetesPod_Container_Resources_EXBIBYTE:
		return "Ei"

	default:
		panic(fmt.Errorf("unknown resource unit (%v)", v))
	}
}
