export interface KeyFrame {
    s: {
        t: string; // text
        mc: number; // max characters
    }
}

export interface LottieLayer extends Record<string, unknown> {
    ty: number; // Type
    nm: string; // Name
    refId: string;
    ind: number;
    t: {
        d: {
            k: KeyFrame[];
        }
    }
}

export interface LottieAsset {
    id: string;
    layers: LottieLayer[];
}

export interface LottieAnimation extends Record<string, unknown> {
    assets: LottieAsset[];
    layers: LottieLayer[];
    w: number;
    h: number;
}
