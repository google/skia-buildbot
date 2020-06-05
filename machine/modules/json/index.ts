
export interface Annotation {
	Message: string;
	User: string;
	Timestamp: string;
}

export interface Description {
	Mode: string;
	Annotation: Annotation;
	Dimensions: { [key: string]: string[] };
	PodName: string;
	KubernetesImage: string;
	ScheduledForDeletion: string;
	PowerCycle: boolean;
	LastUpdated: string;
	Battery: number;
	Temperature: { [key: string]: number };
	RunningSwarmingTask: boolean;
}
