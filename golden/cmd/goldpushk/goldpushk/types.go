package goldpushk

import (
	"fmt"
	"sort"
)

type GoldInstance string
type GoldService string

// Represents Gold instance/service pair.
type GoldInstanceServicePair struct {
	Instance GoldInstance
	Service  GoldService
}

// Contains all the necessary information to deploy a service for a specific Gold instance.
// TODO(lovisolo): Add any missing fields.
type GoldServiceDeployment struct {
	GoldInstanceServicePair
	Internal      bool // If true, deploy to the "skia-corp" cluster, otherwise deploy to "skia-public".
	ConfigMapName string
}

// GoldServiceDeployment constructor.
func NewGoldServiceDeployment(instance GoldInstance, service GoldService) GoldServiceDeployment {
	return GoldServiceDeployment{
		GoldInstanceServicePair: GoldInstanceServicePair{
			Instance: instance,
			Service:  service,
		},
	}
}

// Returns the deployment's canonical name, used e.g. to name its corresponding .yaml file.
func (d GoldServiceDeployment) CanonicalName() string {
	return fmt.Sprintf("gold-%s-%s", d.Instance, d.Service)
}

// Maps Gold instances to their required services and deployment information.
// Used as the source of truth for all things instances/services.
type GoldServicesMap map[GoldInstance]map[GoldService]GoldServiceDeployment

// Utility function to put items in the map.
func (m *GoldServicesMap) AddDeployment(instance GoldInstance, service GoldService, deployment GoldServiceDeployment) {
	if (*m)[instance] == nil {
		(*m)[instance] = make(map[GoldService]GoldServiceDeployment)
	}

	deployment.Instance = instance
	deployment.Service = service
	(*m)[instance][service] = deployment
}

// Utility method to iterate over all instance/service pairs.
func (m GoldServicesMap) ForAll(f func(GoldInstance, GoldService, GoldServiceDeployment)) {
	// Sort keys first to iterate over them later in a deterministic order.
	instances := make([]GoldInstance, 0)
	for instance := range m {
		instances = append(instances, instance)
	}
	sort.Slice(instances, func(i, j int) bool {
		return instances[i] < instances[j]
	})

	// Iterate over all instances.
	for _, instance := range instances {

		// Sort services for the current instance.
		services := make([]GoldService, 0)
		for service := range m[instance] {
			services = append(services, service)
		}
		sort.Slice(services, func(i, j int) bool {
			return services[i] < services[j]
		})

		// Iterate over all services for the current instance
		for _, service := range services {
			f(instance, service, m[instance][service])
		}
	}
}
