export interface KeyFrame {
    s: {
        t: string; // text
        mc: number; // max characters
    }
}

export interface LottieLayer extends Record<string, unknown> {
    ty: number; // Type
    nm: string; // Name
    refId?: string;
    ind: number;
    t?: {
        d: {
            k: KeyFrame[];
        }
    }
}

export interface LottieAsset {
    id: string;
    layers: LottieLayer[];
    p: string; // path
    u: string; // directory
    w: number;
    h: number;
}

export interface FontAsset {
    fName: string;
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
