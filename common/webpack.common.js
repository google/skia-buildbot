const path = require('path');

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
