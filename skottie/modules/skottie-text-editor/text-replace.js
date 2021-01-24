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
