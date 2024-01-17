import {
  LottieAnimation,
  LottieColorSlot,
  LottieVectorSlot,
  LottieScalarSlot,
  LottieImageSlot,
} from '../types';
import { hexToColor } from '../helpers/color';

export function replaceColorSlot(
  color: string,
  opacity: number,
  sid: string,
  currentAnimation: LottieAnimation
): LottieAnimation {
  // Deep clone the JSON
  const animation = JSON.parse(
    JSON.stringify(currentAnimation)
  ) as LottieAnimation;

  if (!animation.slots?.[sid]) {
    return animation;
  }

  const rgba: number[] = [...hexToColor(color), opacity / 100];

  const colorSlot = animation.slots[sid] as LottieColorSlot;
  colorSlot.p.k = rgba;

  return animation;
}

export function replaceScalarSlot(
  val: number,
  sid: string,
  currentAnimation: LottieAnimation
): LottieAnimation {
  // Deep clone the JSON
  const animation = JSON.parse(
    JSON.stringify(currentAnimation)
  ) as LottieAnimation;

  if (!animation.slots?.[sid]) {
    return animation;
  }

  const scalarSlot = animation.slots[sid] as LottieScalarSlot;
  scalarSlot.p.k = val;

  return animation;
}

export function replaceVec2Slot(
  val: number[],
  sid: string,
  currentAnimation: LottieAnimation
): LottieAnimation {
  // Deep clone the JSON
  const animation = JSON.parse(
    JSON.stringify(currentAnimation)
  ) as LottieAnimation;

  if (!animation.slots?.[sid]) {
    return animation;
  }

  const vectorSlot = animation.slots[sid] as LottieVectorSlot;
  vectorSlot.p.k[0] = val[0];
  vectorSlot.p.k[1] = val[1];

  return animation;
}

export function replaceImageSlot(
  val: string,
  sid: string,
  currentAnimation: LottieAnimation
): LottieAnimation {
  // Deep clone the JSON
  const animation = JSON.parse(
    JSON.stringify(currentAnimation)
  ) as LottieAnimation;

  if (!animation.slots?.[sid]) {
    return animation;
  }

  const imageSlot = animation.slots[sid] as LottieImageSlot;
  imageSlot.p.p = val;

  return animation;
}
