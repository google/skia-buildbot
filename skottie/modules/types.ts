export interface TextInfo extends Record<string, unknown> {
  t: string; // text
  mc?: number; // max characters
  f: string; // font name
  tr: number; // tracking
  lh: number; // lineHeight
}

export interface TextKeyFrame {
  s: TextInfo;
  t: number;
}

export interface MultiDimensionalProperty {
  k: number[];
  a: number; // is property animated (0 for no, 1 for yes)
  ix?: number; // Property index number
}

export interface OneDimensionalProperty {
  k: number;
  a: number; // is property animated (0 for no, 1 for yes)
  ix?: number; // Property index number
}

export interface EditableText extends Record<string, unknown> {
  d: {
    k: TextKeyFrame[];
  };
}

type LottieSliderEffectType = 0;
type LottieColorEffectType = 2;
type LottieTintEffectType = 20;
type LottieEffectType = LottieSliderEffectType | LottieColorEffectType | LottieTintEffectType;

export interface LottieBaseEffect {
  ty: LottieEffectType;
  ef?: LottieBaseEffect[];
  nm?: string; // Name of the effect (this name can be modified by the user)
  mn: string; // Match name of the effect (this is a qualified unmodifiable name)
  ix?: number; // Property index number
}

export interface LottieColorEffect extends LottieBaseEffect {
  ty: LottieColorEffectType;
  v: MultiDimensionalProperty;
}

export interface LottieSliderEffect extends LottieBaseEffect {
  ty: LottieSliderEffectType;
  v: OneDimensionalProperty;
}

export interface LottieTintEffect extends LottieBaseEffect {
  np: number; // Number of nested properties
  en: number; // Enabled (0 / 1)
  ty: LottieTintEffectType;
  ef: [LottieColorEffect, LottieColorEffect, LottieSliderEffect];
}

type LottieEffect = LottieTintEffect | LottieColorEffect;

export interface LottieLayer extends Record<string, unknown> {
  ty: number; // Type
  nm: string; // Name
  refId?: string;
  ind: number;
  t?: EditableText;
  ef?: LottieEffect[];
}

export interface LottieBinaryAsset {
  id: string;
  p: string; // path
  u?: string; // directory
  w?: number;
  h?: number;
  nm?: string; // name
  sid?: string; // slot id
}

export interface LottieCompAsset {
  layers: LottieLayer[];
  id: string;
  fr: number; // frame rate
}

export type LottieAsset = LottieBinaryAsset | LottieCompAsset;

export interface FontAsset extends Record<string, unknown> {
  fName: string;
  fFamily: string;
  fStyle: string;
}

export interface LottieColorSlot {
  p: MultiDimensionalProperty;
  t: number;
}

export interface LottieVectorSlot {
  p: MultiDimensionalProperty;
  t: number;
}

export interface LottieScalarSlot {
  p: OneDimensionalProperty;
  t: number;
}

export interface LottieImageSlot {
  t: number;
  p: {
    id: string;
    w: number;
    h: number;
    u: string;
    p: string;
    e: number;
  };
}

export type LottieSlot = LottieColorSlot | LottieVectorSlot | LottieScalarSlot | LottieImageSlot;

export interface LottieAnimation extends Record<string, unknown> {
  assets: LottieAsset[];
  layers: LottieLayer[];
  fonts?: {
    list?: FontAsset[];
  };
  metadata?: {
    filename?: string;
  };
  w: number;
  h: number;
  fr?: number;
  slots?: Record<string, LottieSlot>;
}

export type ViewMode = 'presentation' | 'default';
