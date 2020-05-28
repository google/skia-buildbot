// this file was automatically generated, DO NOT EDIT
// classes
// struct2ts:go.skia.org/infra/machine/go/machine.DescriptionAnnotation
interface DescriptionAnnotation {
	Message: string;
	User: string;
	Timestamp: Date;
}

// struct2ts:go.skia.org/infra/machine/go/machine.Description
interface Description {
	Mode: string;
	Annotation: DescriptionAnnotation;
	Dimensions: { [key: string]: []string };
	PodName: string;
	KubernetesImage: string;
	ScheduledForDeletion: string;
	PowerCycle: boolean;
	LastUpdated: Date;
	Battery: number;
	Temperature: { [key: string]: number };
	RunningSwarmingTask: boolean;
}

// exports
export {
	DescriptionAnnotation,
	Description,
};
