const path = require('path');

module.exports = {
  plugins: [path.resolve(__dirname, './plugins/element')],
  source: {
    includePattern: "modules\\/.+\\.js$",
    excludePattern: "(^|\\/|\\\\)_"
  },
};
