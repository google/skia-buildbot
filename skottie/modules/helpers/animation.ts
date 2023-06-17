// Since LottieAssets don't have a single field to do discriminated unions,
// we are using type guards to narrow down Lottie asset types.
// link with some helpful information
// https://www.typescriptlang.org/docs/handbook/2/narrowing.html#using-type-predicates

import { LottieAsset, LottieCompAsset, LottieBinaryAsset } from '../types';

const isCompAsset = (asset: LottieAsset): asset is LottieCompAsset => {
  return 'layers' in asset;
};

const isBinaryAsset = (asset: LottieAsset): asset is LottieBinaryAsset => {
  return 'p' in asset;
};

export { isCompAsset, isBinaryAsset };
