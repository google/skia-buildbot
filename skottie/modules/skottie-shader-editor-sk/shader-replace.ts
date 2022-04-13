import { LottieAnimation, LottieLayer } from '../types';

export interface ExtraLayerData {
  layer: LottieLayer;
  parentId: string;
  precompName: string;
}

export interface ShaderData {
  id: string;
  name: string;
  shader: string;
  precompName: string;
  items: ExtraLayerData[];
}

//TODO(jmbetancourt): return LottieAnimation with replaced shaders
export const replaceShaders = (texts: ShaderData[],
                               currentAnimation: LottieAnimation): LottieAnimation => {
  return currentAnimation;
};