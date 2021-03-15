export const replaceTexts = (texts, currentAnimation) => {
  const animation = JSON.parse(JSON.stringify(currentAnimation));
  texts.forEach((textData) => {
    textData.items.forEach((item) => {
      let layers;
      // Searches for composition that contains this layer
      if (!item.parentId) {
        layers = animation.layers;
      } else {
        const asset = animation.assets.find((assetItem) => assetItem.id === item.parentId);
        layers = asset ? asset.layers : [];
      }

      // Replaces current animation layer with new layer value
      layers.forEach((layer, index) => {
        if (layer.ind === item.layer.ind && layer.nm === item.layer.nm) {
          layers[index] = item.layer;
        }
      });
    });
  });
  return animation;
};

const replaceTextsInLayers = (textsDictionary, layers) => {
  const LAYER_TEXT_TYPE = 5;
  layers.forEach((layer) => {
    if (layer.ty === LAYER_TEXT_TYPE && textsDictionary[layer.nm]) {
      // It's read as: Layer > Text Element > Text document > First Keyframe > Start Value > Text
      layer.t.d.k[0].s.t = textsDictionary[layer.nm];
    }
  });
};

export const replaceTextsByLayerName = (texts, currentAnimation) => {
  if (!texts) {
    return currentAnimation;
  }
  const animation = JSON.parse(JSON.stringify(currentAnimation));
  // Create dictionary to access data by name instead of iterating on every layer
  const textsDictionary = texts.reduce((dict, text) => {
    dict[text.name] = text.text;
    return dict;
  }, {});
  replaceTextsInLayers(textsDictionary, animation.layers);

  animation.assets
    .filter((asset) => asset.layers)
    .forEach((asset) => replaceTextsInLayers(textsDictionary, asset.layers));

  return animation;
};
