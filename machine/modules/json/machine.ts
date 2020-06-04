// this file was automatically generated, DO NOT EDIT

// helpers
const maxUnixTSInSeconds = 9999999999;

function ParseDate(d: Date | number | string): Date {
	if (d instanceof Date) return d;
	if (typeof d === 'number') {
		if (d > maxUnixTSInSeconds) return new Date(d);
		return new Date(d * 1000); // go ts
	}
	return new Date(d);
}

function ParseNumber(v: number | string, isInt = false): number {
	if (!v) return 0;
	if (typeof v === 'number') return v;
	return (isInt ? parseInt(v) : parseFloat(v)) || 0;
}

function FromArray<T>(Ctor: { new(v: any): T }, data?: any[] | any, def = null): T[] | null {
	if (!data || !Object.keys(data).length) return def;
	const d = Array.isArray(data) ? data : [data];
	return d.map((v: any) => new Ctor(v));
}

function ToObject(o: any, typeOrCfg: any = {}, child = false): any {
	if (!o) return null;
	if (typeof o.toObject === 'function' && child) return o.toObject();

	switch (typeof o) {
		case 'string':
			return typeOrCfg === 'number' ? ParseNumber(o) : o;
		case 'boolean':
		case 'number':
			return o;
	}

	if (o instanceof Date) {
		return typeOrCfg === 'string' ? o.toISOString() : Math.floor(o.getTime() / 1000);
	}

	if (Array.isArray(o)) return o.map((v: any) => ToObject(v, typeOrCfg, true));

	const d: any = {};

	for (const k of Object.keys(o)) {
		const v: any = o[k];
		if (!v) continue;
		d[k] = ToObject(v, typeOrCfg[k] || {}, true);
	}

	return d;
}

// classes
// struct2ts:go.skia.org/infra/machine/go/machine.DescriptionAnnotation
class DescriptionAnnotation {
	Message: string;
	User: string;
	Timestamp: Date;

	constructor(data?: any) {
		const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
		this.Message = ('Message' in d) ? d.Message as string : '';
		this.User = ('User' in d) ? d.User as string : '';
		this.Timestamp = ('Timestamp' in d) ? ParseDate(d.Timestamp) : new Date();
	}

	toObject(): any {
		const cfg: any = {};
		cfg.Timestamp = 'string';
		return ToObject(this, cfg);
	}
}

// struct2ts:go.skia.org/infra/machine/go/machine.Description
class Description {
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

	constructor(data?: any) {
		const d: any = (data && typeof data === 'object') ? ToObject(data) : {};
		this.Mode = ('Mode' in d) ? d.Mode as string : '';
		this.Annotation = new DescriptionAnnotation(d.Annotation);
		this.Dimensions = ('Dimensions' in d) ? d.Dimensions as { [key: string]: []string } : {};
		this.PodName = ('PodName' in d) ? d.PodName as string : '';
		this.KubernetesImage = ('KubernetesImage' in d) ? d.KubernetesImage as string : '';
		this.ScheduledForDeletion = ('ScheduledForDeletion' in d) ? d.ScheduledForDeletion as string : '';
		this.PowerCycle = ('PowerCycle' in d) ? d.PowerCycle as boolean : false;
		this.LastUpdated = ('LastUpdated' in d) ? ParseDate(d.LastUpdated) : new Date();
		this.Battery = ('Battery' in d) ? d.Battery as number : 0;
		this.Temperature = ('Temperature' in d) ? d.Temperature as { [key: string]: number } : {};
		this.RunningSwarmingTask = ('RunningSwarmingTask' in d) ? d.RunningSwarmingTask as boolean : false;
	}

	toObject(): any {
		const cfg: any = {};
		cfg.LastUpdated = 'string';
		cfg.Battery = 'number';
		return ToObject(this, cfg);
	}
}

// exports
export {
	DescriptionAnnotation,
	Description,
	ParseDate,
	ParseNumber,
	FromArray,
	ToObject,
};
