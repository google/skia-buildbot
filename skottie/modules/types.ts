export interface TextInfo extends Record<string, unknown> {
    t: string; // text
    mc?: number; // max characters
}

export interface TextKeyFrame {
    s: TextInfo;
    t: number;
}

export interface EditableText extends Record<string, unknown> {
    d: {
        k: TextKeyFrame[];
    }
}

export interface LottieLayer extends Record<string, unknown> {
    ty: number; // Type
    nm: string; // Name
    refId?: string;
    ind: number;
    t?: EditableText;
}

export interface LottieAsset {
    id: string;
    layers: LottieLayer[];
    p: string; // path
    u: string; // directory
    w: number;
    h: number;
}

export interface FontAsset extends Record<string, unknown> {
    fName: string;
    fFamily: string;
}

export interface LottieAnimation extends Record<string, unknown> {
    assets: LottieAsset[];
    layers: LottieLayer[];
    fonts?: {
        list?: FontAsset[];
    };
    metadata?: {
        filename?: string;
    }
    w: number;
    h: number;
    fr?: number;
}

export type ViewMode = 'presentation' | 'default';
