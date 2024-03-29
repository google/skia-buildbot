// DO NOT EDIT. This file is automatically generated.

export interface SVGMetaData {
}

export interface ChildShader {
	UniformName: string;
	ScrapHashOrName: string;
}

export interface SKSLMetaData {
	Uniforms: number[] | null;
	ImageURL: string;
	Children: ChildShader[] | null;
}

export interface ParticlesMetaData {
}

export interface ScrapBody {
	Type: Type;
	Body: string;
	SVGMetaData?: SVGMetaData | null;
	SKSLMetaData?: SKSLMetaData | null;
	ParticlesMetaData?: ParticlesMetaData | null;
}

export interface ScrapID {
	Hash: SHA256;
}

export interface SkShadersConfig {
	fiddle_origin: string;
	jsfiddle_origin: string;
}

export type Type = 'svg' | 'sksl' | 'particle';

export type SHA256 = string;
