// DO NOT EDIT. This file is automatically generated.

export interface Note {
	text: string;
	author: string;
	ts: number;
}

export interface Incident {
	key: string;
	id: string;
	active: boolean;
	start: number;
	last_seen: number;
	params: { [key: string]: string };
	notes: Note[] | null;
}

export interface Silence {
	key: string;
	active: boolean;
	user: string;
	param_set: ParamSet;
	created: number;
	updated: number;
	duration: string;
	notes: Note[] | null;
}

export interface RecentIncidentsResponse {
	incidents: Incident[] | null;
	flaky: boolean;
	recently_expired_silence: boolean;
}

export interface StatsRequest {
	range: string;
}

export interface Stat {
	num: number;
	incident: Incident;
}

export interface IncidentsResponse {
	incidents: Incident[] | null;
	ids_to_recently_expired_silences: { [key: string]: boolean };
}

export interface IncidentsInRangeRequest {
	range: string;
	incident: Incident;
}

export type ParamSet = { [key: string]: string[] | null };

export type Params = { [key: string]: string };

export type StatsResponse = (Stat | null)[] | null;
