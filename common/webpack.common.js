const path = require('path');

// The addCommon function adds an alias for each of the
// well known libraries of custom elements and modules.
module.exports.addCommon = function(webpack_config) {
  webpack_config.resolve = {
    alias: {
      // Keep these libraries at well known locations.
      'skia-elements': path.resolve(__dirname, '..', 'ap', 'skia-elements'),
      'common': path.resolve(__dirname, 'modules'),
    },
  };
  return webpack_config;
}
