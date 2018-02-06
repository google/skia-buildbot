const path = require('path');
const finder = require('../../../res/mod/demo-finder.js')

let common = require('../../../res/mod/webpack.common.js');
common.output.path = path.resolve(__dirname, 'dist')

// Use finder to auto-populate the demos into the webpack_config.
common = finder(__dirname, common);

module.exports = common;
