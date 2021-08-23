import { LottieAnimation, LottieAsset, LottieLayer } from '../types';

export interface ExtraLayerData {
  layer: LottieLayer;
  parentId: string;
  precompName: string;
}

export interface TextData {
  id: string;
  name: string;
  text: string;
  maxChars?: number; // If not defined, we don't constrain the <textarea>
  precompName: string;
  items: ExtraLayerData[];
}

export const replaceTexts = (texts: TextData[], currentAnimation: LottieAnimation): LottieAnimation => {
  const animation = JSON.parse(JSON.stringify(currentAnimation)) as LottieAnimation;
  texts.forEach((textData) => {
    textData.items.forEach((item: ExtraLayerData) => {
      let layers;
      // Searches for composition that contains this layer
      if (!item.parentId) {
        layers = animation.layers;
      } else {
        const asset = animation.assets.find((assetItem: LottieAsset) => assetItem.id === item.parentId);
        layers = asset ? asset.layers : [];
      }

      // Replaces current animation layer with new layer value
      layers.forEach((layer: LottieLayer, index: number) => {
        if (layer.ind === item.layer.ind && layer.nm === item.layer.nm) {
          layers[index] = item.layer;
        }
      });
    });
  });
  return animation;
};

const replaceTextsInLayers = (textsDictionary: Record<string, string>, layers: LottieLayer[]) => {
  const LAYER_TEXT_TYPE = 5;
  layers.forEach((layer: LottieLayer) => {
    if (layer.ty === LAYER_TEXT_TYPE && textsDictionary[layer.nm]) {
      // It's read as: Layer > Text Element > Text document > First Keyframe > Start Value > Text
      const textElement: any = layer.t;
      textElement.d.k[0].s.t = textsDictionary[layer.nm];
    }
  });
};

export const replaceTextsByLayerName = (texts: TextData[], currentAnimation: LottieAnimation): LottieAnimation => {
  if (!texts) {
    return currentAnimation;
  }
  // Make a copy of the original animation.
  const animation: LottieAnimation = JSON.parse(JSON.stringify(currentAnimation));
  // Create dictionary to access data by name instead of iterating on every layer
  const textsDictionary = texts.reduce((dict: Record<string, string>, text: TextData) => {
    dict[text.name] = text.text;
    return dict;
  }, {});
  replaceTextsInLayers(textsDictionary, animation.layers);

  animation.assets
    .filter((asset: LottieAsset) => asset.layers)
    .forEach((asset: LottieAsset) => replaceTextsInLayers(textsDictionary, asset.layers));

  return animation;
};
