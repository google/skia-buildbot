const path = require('path');
const finder = require('../../../res/mod/demo-finder.js')

const common_builder = require('../../../res/mod/webpack.common.js');
let common = common_builder(__dirname);

// Use finder to auto-populate the demos into the webpack_config.
common = finder(__dirname, common);

module.exports = common;
