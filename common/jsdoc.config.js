'use strict';

module.exports = {
  plugins: ['./plugins/element'],
  source: {
    includePattern: "modules\\/.+\\.js$",
    excludePattern: "(^|\\/|\\\\)_"
  },
};
